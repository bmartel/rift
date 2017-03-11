package rift

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc"

	"github.com/bmartel/rift/summary"
	"github.com/satori/go.uuid"
	"github.com/uber-go/zap"
)

// Queue provides the context and handling for jobs for one App instance
type Queue struct {
	id string

	// A buffered channel that we can send work requests on.
	channel chan ReservedJob

	// workers channel
	workers         chan *Worker
	reservedWorkers chan *Worker
	removed         chan bool
	quit            chan bool

	// metrics channel
	closeMetric   chan bool
	removedMetric chan bool
	metric        chan *summary.Job

	// metrics
	stats *summary.Stats

	createdAt time.Time

	// logging
	logger  zap.Logger
	verbose bool

	// monitoring
	statsAddr  string
	rpcConn    *grpc.ClientConn
	monitoring summary.SummaryClient
}

// Options provides a way to configure a rift queue
type Options struct {
	Tag       string
	Service   Service
	Workers   int
	Queues    int
	Verbose   bool
	StatsAddr string
}

// New creates a rift queue, allowing options to be passed
func New(opts *Options) *Queue {
	// Number of workers to spawn
	maxWorkers, err := strconv.Atoi(maxWorker)
	if err != nil {
		maxWorkers = 100
	}
	// Size of job Queue
	maxQueues, err := strconv.Atoi(maxQueue)
	if err != nil {
		maxQueues = 100
	}

	if opts == nil {
		opts = &Options{"", nil, 0, 0, false, ""}
	}

	if opts.Tag == "" {
		opts.Tag = uuid.NewV4().String()
	}
	if opts.Workers < 1 {
		opts.Workers = maxWorkers
	}
	if opts.Queues < 1 {
		opts.Queues = maxQueues
	}

	var id uuid.UUID
	id = uuid.NewV4()
	q := &Queue{
		id:              id.String(),
		channel:         make(chan ReservedJob, opts.Queues),
		workers:         make(chan *Worker, opts.Workers),
		reservedWorkers: make(chan *Worker, opts.Workers),
		removed:         make(chan bool),
		quit:            make(chan bool),
		closeMetric:     make(chan bool),
		removedMetric:   make(chan bool),
		metric:          make(chan *summary.Job),
		stats:           new(summary.Stats),
		statsAddr:       opts.StatsAddr,
		logger:          zap.New(zap.NewJSONEncoder(), zap.Fields(zap.String("queue", id.String()))),
		verbose:         opts.Verbose,
		createdAt:       time.Now(),
	}

	q.stats.App = opts.Tag
	q.stats.QueueId = q.id
	q.stats.Jobs = make(map[string]*summary.Job, 0)

	if opts.Verbose {
		q.logger.Info("queue started")
	}

	// starting n number of workers
	for i := 0; i < opts.Workers; i++ {
		id = dispatchWorker(opts.Service, q.workers, q.channel, q.reservedWorkers, q.metric, opts.Queues, q.logger, opts.Verbose)
	}

	q.logger.Info("workers started", zap.Int("count", opts.Workers))

	go q.dispatch()
	go q.metrics()

	q.StartMonitoring()

	return q
}

// Stats of this queue's operation metrics
func (q *Queue) Stats() *summary.Stats {
	return q.stats
}

// StartMonitoring tries a connection to a monitoring server instance if available
func (q *Queue) StartMonitoring() {
	if q.rpcConn == nil && q.statsAddr != "" {
		conn, err := grpc.Dial(q.statsAddr, grpc.WithInsecure())
		if err != nil {
			q.logger.Error(err.Error())
		} else {
			q.rpcConn = conn
			q.monitoring = summary.NewSummaryClient(q.rpcConn)
			q.logger.Info("CONNECTED TO MONITORING SERVER")
		}
	}
}

// Later queues up a job for processing and returns the id of the job
func (q *Queue) Later(job Job, retry uint8) uuid.UUID {
	id := uuid.NewV4()

	q.channel <- ReservedJob{
		ID:          id,
		Job:         job,
		RequestedAt: time.Now(),
		Retry:       retry,
	}

	q.logger.Info("job queued", zap.String("job", id.String()))

	q.metric <- &summary.Job{Id: id.String(), Status: "job.queued", Worker: q.id}

	return id
}

// Close the queue, first draining any open workers and jobs in queue
func (q *Queue) Close() {
	q.quit <- true
	<-q.removed
	q.drain()
	close(q.removed)
	close(q.workers)
	close(q.reservedWorkers)
	close(q.channel)
	q.closeMetric <- true
	<-q.removedMetric
	close(q.removedMetric)
	q.monitoring = nil
	if q.rpcConn != nil {
		q.rpcConn.Close()
	}
	if q.verbose {
		q.logger.Info("queue stopped")
	}
}

func (q *Queue) dispatch() {
	if q.verbose {
		q.logger.Info("queue started")
	}

	for {
		select {
		case job := <-q.channel:
			// a job request has been received
			// try to obtain a worker that is available.
			worker := <-q.reservedWorkers

			// dispatch the job to the worker channel
			worker.channel <- job

		case <-q.quit:
			close(q.quit)
			q.removed <- true
			return
		}
	}
}

func (q *Queue) metrics() {
	for {
		select {
		case job := <-q.metric:
			updateJob(q.stats, job)
			q.StartMonitoring()
			if q.monitoring != nil {
				q.monitoring.UpdateStats(context.Background(), q.stats)
			}
		case <-q.closeMetric:
			close(q.metric)
			close(q.closeMetric)
			q.removedMetric <- true
			return
		}
	}
}

func (q *Queue) drain() {
	for {
		select {
		case worker := <-q.workers:
			worker.Close()
		default:
			return
		}
	}
}
