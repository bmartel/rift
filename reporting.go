package rift

import (
	"time"
)

type Stats struct {
	QueueID string   `json:"queue_id"`
	Workers []string `json:"workers"`

	ActiveJobs    uint8  `json:"active_jobs"`
	QueuedJobs    uint32 `json:"queued_jobs"`
	ProcessedJobs uint32 `json:"processed_jobs"`
	DeferredJobs  uint32 `json:"deferred_jobs"`
	FailedJobs    uint32 `json:"failed_jobs"`
	RequeuedJobs  uint32 `json:"requeued_jobs"`

	CreatedAt time.Time `json:"created_at"`
}

type Metric interface {
	Type() string
	Value() interface{}
}

type QueueMetric string

func (m QueueMetric) Type() string {
	return string(m)
}
func (m QueueMetric) Value() interface{} {
	return 1
}
