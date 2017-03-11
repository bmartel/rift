package rift_test

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/bmartel/rift"

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
	return "Sample Job"
}

func (t SampleJob) Process(service rift.Service) error {
	log.Printf("ID: %d Title: %s Body: %s\n", t.ID, t.Title, t.Body)
	return nil
}

type FailedJob struct {
}

func (t FailedJob) Tag() string {
	return "Failing Job"
}

func (t FailedJob) Process(service rift.Service) error {
	if connectionRetry < 2 {
		connectionRetry++
		return fmt.Errorf("connection timeout error")
	}
	return nil
}

var _ = Describe("Queue", func() {
	var (
		queue *rift.Queue
	)

	BeforeEach(func() {
		connectionRetry = 0

		log.Println(runtime.NumGoroutine())
		queue = rift.New(&rift.Options{"Test", nil, 10, 10, false, "localhost:9147"})
	})
	AfterEach(func() {
		queue.Close()
		log.Println(runtime.NumGoroutine())
	})

	Describe("Queueing a job", func() {
		It("should correctly queue and process a single job", func(done Done) {
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)

			time.Sleep(time.Second * 1)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(1)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))
			close(done)
		}, 3)

		It("should correctly queue and process multiple jobs", func(done Done) {
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)
			queue.Later(SampleJob{1, "Rift", "Running a Managed Goroutine"}, 0)

			time.Sleep(time.Second * 1)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(2)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(2)))
			Expect(stats.FailedJobs).To(Equal(uint32(0)))

			close(done)
		}, 3)

		It("should discard a failed job when retry is set to 0", func(done Done) {
			queue.Later(FailedJob{}, 0)

			time.Sleep(time.Second * 1)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(0)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(0)))
			Expect(stats.FailedJobs).To(Equal(uint32(1)))

			close(done)
		}, 3)

		It("should requeue a job up to the set retry limit", func(done Done) {
			queue.Later(FailedJob{}, 1)

			time.Sleep(time.Second * 1)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(0)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(1)))
			Expect(stats.FailedJobs).To(Equal(uint32(2)))

			close(done)
		}, 3)

		It("should requeue a job and succeed if within retry limit and without error", func(done Done) {
			queue.Later(FailedJob{}, 3)

			time.Sleep(time.Second * 1)

			stats := queue.Stats()
			Expect(stats.QueuedJobs).To(Equal(uint32(1)))
			Expect(stats.ProcessedJobs).To(Equal(uint32(1)))
			Expect(stats.RequeuedJobs).To(Equal(uint32(2)))
			Expect(stats.FailedJobs).To(Equal(uint32(2)))

			close(done)
		}, 3)
	})
})
