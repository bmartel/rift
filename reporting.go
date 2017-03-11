package rift

import "github.com/bmartel/rift/summary"

func updateJob(s *summary.Stats, job *summary.Job) {
	s.Jobs[job.Id] = job

	switch job.Status {
	case "job.queued":
		s.QueuedJobs++
	case "job.started":
		s.ActiveJobs++
	case "job.processed":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.ProcessedJobs++
	case "job.failed":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.FailedJobs++
	case "job.deferred":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.DeferredJobs++
	case "job.requeued":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.RequeuedJobs++
	}
}
