package rift

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/grpc"

	"github.com/bmartel/rift/summary"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

// Queue provides the context and handling for jobs for one App instance
type Queue struct {
	id string

	// A buffered channel that we can send work requests on.
	channel      chan ReservedJob
	closeQueue   chan bool
	queueRemoved chan bool

	// workers channel
	workers chan *Worker

	// metrics channel
	metrics              chan *summary.Job
	closeMetricsServer   chan bool
	metricsServerRemoved chan bool

	// metrics
	stats *summary.Stats

	createdAt time.Time

	// logging
	logger  *zap.Logger
	verbose bool

	// serialization
	registry *Registry

	// monitoring
	statsAddr  string
	rpcConn    *grpc.ClientConn
	monitoring summary.SummaryClient
}

// Options provides a way to configure a rift queue
type Options struct {
	Tag       string
	Workers   int
	Queues    int
	Verbose   bool
	StatsAddr string
}

// New creates a rift queue, allowing options to be passed
func New(opts *Options, service Service) *Queue {

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
		opts = &Options{"", 0, 0, false, ""}
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

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()

	q := &Queue{
		id:                   id.String(),
		channel:              make(chan ReservedJob, opts.Queues),
		workers:              make(chan *Worker, opts.Workers),
		closeQueue:           make(chan bool),
		queueRemoved:         make(chan bool),
		closeMetricsServer:   make(chan bool),
		metricsServerRemoved: make(chan bool),
		metrics:              make(chan *summary.Job),
		stats:                new(summary.Stats),
		registry:             NewRegistry(),
		statsAddr:            opts.StatsAddr,
		logger:               logger,
		verbose:              opts.Verbose,
		createdAt:            time.Now(),
	}

	q.stats.App = opts.Tag
	q.stats.QueueId = q.id
	q.stats.Jobs = make(map[string]*summary.Job, 0)
	q.stats.JobBlueprints = make([]*summary.JobBlueprint, 0)

	if opts.Verbose {
		q.logger.Info("queue started")
	}

	// starting n number of workers
	for i := 0; i < opts.Workers; i++ {
		dispatchWorker(q, service, opts)
	}

	q.logger.Info("workers started", zap.Int("count", opts.Workers))

	go q.startDispatcher()
	go q.startMetricsCapture()

	return q
}

// Stats of this queue's operation metrics
func (q *Queue) Stats() *summary.Stats {
	return q.stats
}

// tries a connection to a monitoring server instance if available
func (q *Queue) startMonitoringServer() {
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
	go func() {
		q.channel <- ReservedJob{
			ID:          id,
			Job:         job,
			RequestedAt: time.Now(),
			Retry:       retry,
		}

		q.logger.Info("job queued", zap.String("job", id.String()))

		q.metrics <- &summary.Job{Id: id.String(), Status: "queued", Worker: q.id}

	}()
	return id
}

// Register a job type so it can be looked up through deserialization
func (q *Queue) Register(jobs ...Job) {
	for _, job := range jobs {
		// Capture the job imprint for serialization
		q.registry.SerializeJob(job, q.stats)
	}
}

// CreateJob type through serialization
func (q *Queue) CreateJob(jobType string, data map[string]interface{}, retry uint8) (string, error) {
	job := q.registry.DeserializeJob(jobType, data)
	if job == nil {
		return "", fmt.Errorf("no job serializer could be found for %s", jobType)
	}

	id := q.Later(job, retry)

	return id.String(), nil
}

// Close the queue, first draining any open workers and jobs in queue
func (q *Queue) Close() {
	q.closeQueue <- true
	<-q.queueRemoved
	q.drain()
	close(q.queueRemoved)
	close(q.workers)
	close(q.channel)
	q.closeMetricsServer <- true
	<-q.metricsServerRemoved
	close(q.metricsServerRemoved)
	q.monitoring = nil
	if q.rpcConn != nil {
		q.rpcConn.Close()
		q.rpcConn = nil
	}
	if q.verbose {
		q.logger.Info("queue stopped")
	}
}

func (q *Queue) startDispatcher() {
	if q.verbose {
		q.logger.Info("queue started")
	}

	for {
		select {
		case job := <-q.channel:
			// a job request has been received
			// try to obtain a worker that is available.
			worker := <-q.workers

			// dispatch the job to the worker channel
			worker.channel <- job

		case <-q.closeQueue:
			close(q.closeQueue)
			q.queueRemoved <- true
			return
		}
	}
}

func (q *Queue) startMetricsCapture() {
	for {
		select {
		case job := <-q.metrics:
			updateJob(q.stats, job)
			q.startMonitoringServer()
			if q.monitoring != nil {
				q.monitoring.UpdateJob(context.Background(), &summary.JobUpdate{App: q.stats.App, QueueId: q.stats.QueueId, Job: job})
			}
		case <-q.closeMetricsServer:
			close(q.metrics)
			close(q.closeMetricsServer)
			q.metricsServerRemoved <- true
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
