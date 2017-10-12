package rift

import (
	"os"
	"time"

	"github.com/bmartel/rift/summary"
	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

var (
	maxWorker = os.Getenv("MAX_WORKERS")
	maxQueue  = os.Getenv("MAX_QUEUE")
)

// Service represents application specific dependencies that need to be passed to a job
type Service interface{}

// Job is the base required interface for queueing new work
type Job interface {
	Tag() string
	Deserialize(data map[string]interface{}) Job
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
	ID      uuid.UUID
	channel chan ReservedJob
	removed chan bool
	quit    chan bool

	queue   *Queue
	service Service

	logger  *zap.Logger
	verbose bool
}

func dispatchWorker(queue *Queue, service Service, opts *Options) uuid.UUID {
	id := uuid.NewV4()
	w := &Worker{
		ID:      id,
		channel: make(chan ReservedJob, opts.Queues),
		removed: make(chan bool),
		quit:    make(chan bool),
		queue:   queue,
		service: service,
		logger:  queue.logger.With(zap.String("worker", id.String())),
		verbose: opts.Verbose,
	}
	go w.Open()
	return id
}

// Open method starts the run loop for the worker, listening for a quit channel in
// case we need to stop
func (w *Worker) Open() {
	if w.verbose {
		w.logger.Info("worker started")
	}

	// register the current worker into the worker queue.
	w.queue.workers <- w

	for {
		select {
		case job := <-w.channel:
			w.queue.metrics <- &summary.Job{Id: job.ID.String(), Tag: job.Job.Tag(), Status: "started", Worker: w.ID.String()}
			w.logger.Info("job started", zap.String("job", job.ID.String()))
			// we have received a work request.
			if err := job.Job.Process(w.service); err != nil {
				w.queue.metrics <- &summary.Job{Id: job.ID.String(), Tag: job.Job.Tag(), Status: "failed", Worker: w.ID.String()}
				w.logger.Error("job failed: "+err.Error(), zap.String("job", job.ID.String()))
				if job.Retry > job.Requeued {
					w.queue.metrics <- &summary.Job{Id: job.ID.String(), Tag: job.Job.Tag(), Status: "requeued", Worker: w.ID.String()}
					w.logger.Info("job requeued", zap.String("job", job.ID.String()))
					// requeue the job
					job.Requeued++
					w.queue.channel <- job
				}
			} else {
				w.queue.metrics <- &summary.Job{Id: job.ID.String(), Tag: job.Job.Tag(), Status: "processed", Worker: w.ID.String()}
				w.logger.Info("job processed", zap.String("job", job.ID.String()), zap.Float64("duration", time.Since(job.RequestedAt).Seconds()))
				// Put the worker back into the queue reserve for another job to use
				w.queue.workers <- w
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
