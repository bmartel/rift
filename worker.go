package rift

import (
	"os"
	"time"

	"github.com/satori/go.uuid"
	"github.com/uber-go/zap"
)

var (
	maxWorker = os.Getenv("MAX_WORKERS")
	maxQueue  = os.Getenv("MAX_QUEUE")
)

type Service interface{}

// Job is the base required interface for queueing new work
type Job interface {
	Process(Service) error
}

// ReservedJob is the serializable job which is queued and consumed
type ReservedJob struct {
	ID          uuid.UUID
	Job         Job
	RequestedAt time.Time
	Retry       uint8
	Requeued    uint8
}

// Worker represents the worker that executes the job
type Worker struct {
	ID       uuid.UUID
	register chan *Worker
	reserve  chan *Worker
	channel  chan ReservedJob
	requeue  chan ReservedJob
	removed  chan bool
	quit     chan bool

	service Service

	metric  chan Metric
	logger  zap.Logger
	verbose bool
}

func dispatchWorker(service Service, register chan *Worker, requeue chan ReservedJob, reserve chan *Worker, metric chan Metric, queueSize int, logger zap.Logger, verbose bool) uuid.UUID {
	id := uuid.NewV4()
	w := &Worker{
		ID:       id,
		register: register,
		reserve:  reserve,
		metric:   metric,
		channel:  make(chan ReservedJob, queueSize),
		requeue:  requeue,
		removed:  make(chan bool),
		quit:     make(chan bool),
		service:  service,
		logger:   logger.With(zap.String("worker", id.String())),
		verbose:  verbose,
	}
	go w.Open()
	return id
}

// Start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop
func (w *Worker) Open() {
	if w.verbose {
		w.logger.Info("worker started")
	}

	// register the current worker into the worker queue.
	w.register <- w
	w.reserve <- w
	for {
		select {
		case job := <-w.channel:
			w.metric <- QueueMetric("job.started")
			w.logger.Info("job started", zap.String("job", job.ID.String()))
			// we have received a work request.
			if err := job.Job.Process(w.service); err != nil {
				w.metric <- QueueMetric("job.failed")
				w.logger.Error("job failed: "+err.Error(), zap.String("job", job.ID.String()))
				if job.Retry > job.Requeued {
					w.metric <- QueueMetric("job.requeued")
					w.logger.Info("job requeued", zap.String("job", job.ID.String()))
					// requeue the job
					job.Requeued++
					w.requeue <- job
				}
			} else {
				w.metric <- QueueMetric("job.processed")
				w.logger.Info("job processed", zap.String("job", job.ID.String()), zap.Float64("duration", time.Since(job.RequestedAt).Seconds()))
				// Put the worker back into the queue reserve for another job to use
				w.reserve <- w
			}
		case <-w.quit:
			close(w.channel)
			close(w.quit)
			w.removed <- true
			return
		}
	}
}

// Close signals the worker to stop listening for work requests.
func (w *Worker) Close() {
	w.quit <- true
	<-w.removed // wait for the worker to exit
	close(w.removed)
	if w.verbose {
		w.logger.Info("worker stopped")
	}
}
