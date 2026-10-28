package custommodels

import (
	"sync"
	"testing"
	"time"
)

func TestStoreGetConfigDoesNotDeadlockWithConcurrentWriters(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store.mu.Lock()
	store.config.Models = []CustomModel{
		{ID: "model-a", Name: "model-a", Enabled: true},
		{ID: "model-b", Name: "model-b", Enabled: true},
	}
	store.mu.Unlock()

	const (
		readerCount = 8
		writerCount = 4
		iterations  = 250
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for range readerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range iterations {
				_ = store.GetConfig()
			}
		}()
	}
	for writer := range writerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := range iterations {
				modelID := "model-a"
				if (i+writer)%2 == 0 {
					modelID = "model-b"
				}
				if err := store.SetActiveModel(modelID); err != nil {
					t.Errorf("SetActiveModel: %v", err)
					return
				}
			}
		}()
	}

	close(start)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("GetConfig deadlocked with concurrent config writers")
	}
}
