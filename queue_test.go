package rift_test

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/bmartel/rift"
	"github.com/bmartel/rift/summary"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var connectionRetry int = 0

type SampleJob struct {
	ID    int
	Title string
	Body  string
}

func (t SampleJob) Tag() string {
	return "SampleJob"
}

func (t SampleJob) Deserialize(data map[string]interface{}) rift.Job {
	return SampleJob{
		ID:    data["id"].(int),
		Title: data["title"].(string),
		Body:  data["body"].(string),
	}
}

func (t SampleJob) Process(service rift.Service) error {
	if t.ID == 0 || t.Title == "" || t.Body == "" {
		return fmt.Errorf("missing data members")
	}

	log.Printf("ID: %d Title: %s Body: %s\n", t.ID, t.Title, t.Body)
	return nil
}

type FailedJob struct {
}

func (t FailedJob) Tag() string {
	return "FailedJob"
}

func (t FailedJob) Deserialize(data map[string]interface{}) rift.Job {
	return FailedJob{}
}

func (t FailedJob) Process(service rift.Service) error {
	if connectionRetry < 2 {
		connectionRetry++
		return fmt.Errorf("connection timeout error")
	}
	return nil
}

type LongRunningJob struct{}

func (t LongRunningJob) Tag() string {
	return "LongRunningJob"
}

func (t LongRunningJob) Deserialize(data map[string]interface{}) rift.Job {
	return LongRunningJob{}
}

func (t LongRunningJob) Process(service rift.Service) error {

	log.Printf("LongRunningJob - started")
	time.Sleep(time.Second * 6)
	log.Printf("LongRunningJob - complete")

	return nil
}

var _ = Describe("Queue", func() {
	var (
		queue *rift.Queue
	)

	BeforeEach(func() {
		connectionRetry = 0

		log.Println(runtime.NumGoroutine())
		queue = rift.New(&rift.Options{"Test", 100, 100, false, "localhost:9147"}, nil)
	})
	AfterEach(func() {
		queue.Close()
		log.Println(runtime.NumGoroutine())
	})

	Describe("Queueing a job", func() {
		It("should correctly queue and process a single job", func(done Done) {
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(1)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))
			close(done)
		}, 3)

		It("should correctly queue and process multiple jobs", func(done Done) {
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(2)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(2)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))

			close(done)
		}, 3)

		It("should discard a failed job when retry is set to 0", func(done Done) {
			queue.Later(FailedJob{}, 0)

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(0)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(0)))
			Expect(stats.FailedJobs).To(Equal(uint32(1)))

			close(done)
		}, 3)

		It("should requeue a job up to the set retry limit", func(done Done) {
			queue.Later(FailedJob{}, 1)

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(0)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(1)))
			Expect(stats.FailedJobs).To(Equal(uint32(2)))

			close(done)
		}, 3)

		It("should requeue a job and succeed if within retry limit and without error", func(done Done) {
			queue.Later(FailedJob{}, 3)

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(1)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(2)))
			Expect(stats.FailedJobs).To(Equal(uint32(2)))

			close(done)
		}, 3)

		It("should capture a queued jobs details and correctly produce a new job through serialization", func(done Done) {
			queue.Register(SampleJob{})

			id, err := queue.CreateJob("SampleJob", map[string]interface{}{
				"id":    2,
				"title": "Queued indirectly through serialization",
				"body":  "This job could have come from anywhere",
			}, 0)

			Expect(id).ToNot(Equal(""))
			Expect(err).To(BeNil())

			time.Sleep(time.Millisecond * 50)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(1)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))

			close(done)
		}, 3)

		It("should capture a queued jobs details and make them available for external sources to consume", func(done Done) {
			queue.Register(SampleJob{})

			time.Sleep(time.Millisecond * 50)

			expected := []*summary.JobBlueprint{
				&summary.JobBlueprint{
					JobName: "SampleJob",
					Fields: map[string]string{
						"id":    "int",
						"title": "string",
						"body":  "string",
					},
				},
			}
			stats := queue.Stats()
			Expect(stats.JobBlueprints).To(Equal(expected))

			close(done)
		}, 3)

		It("should queue and process jobs even if the queue is saturated", func(done Done) {
			queue2 := rift.New(&rift.Options{"Test", 2, 1, false, "localhost:9147"}, nil)
			queue2.Later(LongRunningJob{}, 1)
			queue2.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)
			queue2.Later(LongRunningJob{}, 1)
			queue2.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)

			time.Sleep(time.Second * 7)

			stats := queue2.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(4)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(4)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(0)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))

			queue2.Close()
			close(done)
		}, 8)

	})
})
