package rift_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

// func TestGoRoutineConsumption(t *testing.T) {
// 	startingGoRoutines := runtime.NumGoroutine()
// 	q := rift.New(&rift.Options{10, 10, false})
// 	time.Sleep(time.Second * 1)
// 	q.Close()
// 	endingGoRoutines := runtime.NumGoroutine()
//
// 	if endingGoRoutines != startingGoRoutines {
// 		t.Errorf("started with %d goroutines and completed with %d", startingGoRoutines, endingGoRoutines)
// 	}
// }

func TestRift(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rift Suite")
}
