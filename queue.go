package rift

import (
	"strconv"
	"time"

	"github.com/satori/go.uuid"
	"github.com/uber-go/zap"
)

type Queue struct {
	id string

	// A buffered channel that we can send work requests on.
	channel chan ReservedJob

	// ids of all workers registered
	workerList []string

	// workers channel
	workers         chan *Worker
	reservedWorkers chan *Worker
	removed         chan bool
	quit            chan bool

	// metrics channel
	closeMetric   chan bool
	removedMetric chan bool
	metric        chan Metric

	// metrics
	active    uint8
	queued    uint32
	processed uint32
	failed    uint32
	deferred  uint32
	requeued  uint32

	createdAt time.Time

	// logging
	logger  zap.Logger
	verbose bool
}

// Options provides a way to configure a rift queue
type Options struct {
	Service Service
	Workers int
	Queues  int
	Verbose bool
}

// New creates a rift queue, allowing options to be passed
func New(opts *Options) *Queue {
	if opts == nil {
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
		opts = &Options{nil, maxWorkers, maxQueues, false}
	}

	var id uuid.UUID
	id = uuid.NewV4()
	q := &Queue{
		id:              id.String(),
		channel:         make(chan ReservedJob, opts.Queues),
		workerList:      make([]string, 0),
		workers:         make(chan *Worker, opts.Workers),
		reservedWorkers: make(chan *Worker, opts.Workers),
		removed:         make(chan bool),
		quit:            make(chan bool),
		closeMetric:     make(chan bool),
		removedMetric:   make(chan bool),
		metric:          make(chan Metric),
		logger:          zap.New(zap.NewJSONEncoder(), zap.Fields(zap.String("queue", id.String()))),
		verbose:         opts.Verbose,
		createdAt:       time.Now(),
	}

	if opts.Verbose {
		q.logger.Info("queue started")
	}

	// starting n number of workers
	for i := 0; i < opts.Workers; i++ {
		id = dispatchWorker(opts.Service, q.workers, q.channel, q.reservedWorkers, q.metric, opts.Queues, q.logger, opts.Verbose)
		q.workerList = append(q.workerList, id.String())
	}

	q.logger.Info("workers started", zap.Int("count", opts.Workers))

	go q.dispatch()
	go q.metrics()

	return q
}

// Stats of this queue's operation metrics
func (q *Queue) Stats() Stats {
	return Stats{
		QueueID:       q.id,
		Workers:       q.workerList,
		ActiveJobs:    q.active,
		QueuedJobs:    q.queued,
		ProcessedJobs: q.processed,
		FailedJobs:    q.failed,
		DeferredJobs:  q.deferred,
		RequeuedJobs:  q.requeued,
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
	q.metric <- QueueMetric("job.queued")

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
		case m := <-q.metric:
			switch m.Type() {
			case "job.queued":
				q.queued++
			case "job.started":
				q.active++
			case "job.processed":
				q.active--
				q.processed++
			case "job.failed":
				q.active--
				q.failed++
			case "job.deferred":
				q.active--
				q.deferred++
			case "job.requeued":
				q.active--
				q.requeued++
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
