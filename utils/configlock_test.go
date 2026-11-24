package utils

import (
	"sync"
	"testing"
)

// The auth middleware calls LoadConfig on every request, concurrently.
func TestLoadConfigConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = LoadConfig()
			}
		}()
	}
	wg.Wait()
}
