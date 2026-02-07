package main

import (
	"sync"
	"testing"
)

func TestEnsureServiceConcurrent(t *testing.T) {
	app := NewApp()
	const workers = 32
	services := make([]any, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			services[i] = app.ensureService()
		}()
	}
	wg.Wait()

	first := services[0]
	if first == nil {
		t.Fatalf("expected non-nil service")
	}
	for i := 1; i < workers; i++ {
		if services[i] != first {
			t.Fatalf("expected same service instance at index %d", i)
		}
	}
}
