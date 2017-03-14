package rift

import "github.com/bmartel/rift/summary"

func updateJob(s *summary.Stats, job *summary.Job) {
	s.Jobs[job.Id] = job

	switch job.Status {
	case "queued":
		s.QueuedJobs++
	case "started":
		s.ActiveJobs++
	case "processed":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.ProcessedJobs++
	case "failed":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.FailedJobs++
	case "deferred":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.DeferredJobs++
	case "requeued":
		if s.ActiveJobs > 0 {
			s.ActiveJobs--
		}
		s.RequeuedJobs++
	}
}
