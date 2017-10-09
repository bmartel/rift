package rift

import (
	"strings"
	"sync"

	"github.com/bmartel/rift/summary"
	"github.com/fatih/structs"
)

// Serializer deconstructs a job and its values
type Serializer struct {
	Job Job `json:"-"`
}

// NewRegistry creates a registry to store serializers
func NewRegistry() *Registry {
	return &Registry{
		serializers: make(map[string]*Serializer, 0),
	}
}

// Registry tracks any in use serializers
type Registry struct {
	mutex       sync.RWMutex
	serializers map[string]*Serializer
}

// SerializeJob deconstructs a job into a serializer and also registers a
// blueprint with the stats service
func (r *Registry) SerializeJob(job Job, stats *summary.Stats) {

	r.mutex.RLock()
	// Check if it already exists
	if _, ok := r.serializers[job.Tag()]; ok {
		r.mutex.RUnlock()
		return // dont reprocess
	}

	r.mutex.RUnlock()

	blueprint := &summary.JobBlueprint{
		JobName: job.Tag(),
		Fields:  make(map[string]string, 0),
	}
	data := structs.New(job)

	for _, field := range data.Fields() {
		tags := strings.Split(field.Tag("json"), ",")
		if tag := tags[0]; tag != "" {
			blueprint.Fields[tag] = field.Kind().String()
		} else {
			blueprint.Fields[strings.ToLower(field.Name())] = field.Kind().String()
		}
	}
	stats.JobBlueprints = append(stats.JobBlueprints, blueprint)

	r.mutex.Lock()
	r.serializers[job.Tag()] = &Serializer{
		Job: job,
	}
	r.mutex.Unlock()
}

// DeserializeJob creates a job from external values
func (r *Registry) DeserializeJob(jobType string, data map[string]interface{}) Job {

	// lookup the incoming job type
	if serializer, ok := r.serializers[jobType]; ok {
		return serializer.Job.Deserialize(data)
	}

	return nil
}
