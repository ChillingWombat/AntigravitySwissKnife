package daemon

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/backend"
)

func TestActiveStateSnapshotCacheHitAndTTL(t *testing.T) {
	var cache activeStateCache
	var builds atomic.Int32
	build := func() backend.ActiveStateSnapshot {
		n := builds.Add(1)
		return backend.ActiveStateSnapshot{ActiveAccount: "one@example.com", TotalAccounts: int(n)}
	}
	first := cache.snapshot(false, build)
	second := cache.snapshot(false, build)
	if builds.Load() != 1 || first.TotalAccounts != 1 || second.TotalAccounts != 1 {
		t.Fatalf("cache hit missed: builds=%d", builds.Load())
	}
	if second.Cache.Hits != 1 || second.Cache.Misses != 1 {
		t.Fatalf("unexpected cache stats: %+v", second.Cache)
	}
	cache.mu.Lock()
	cache.builtAt = time.Now().Add(-activeStateCacheTTL - time.Millisecond)
	cache.mu.Unlock()
	third := cache.snapshot(false, build)
	if builds.Load() != 2 || third.TotalAccounts != 2 {
		t.Fatalf("TTL expiry did not rebuild: builds=%d", builds.Load())
	}
}

func TestActiveStateSnapshotCacheSingleflight(t *testing.T) {
	var cache activeStateCache
	started := make(chan struct{})
	release := make(chan struct{})
	var builds atomic.Int32
	build := func() backend.ActiveStateSnapshot {
		if builds.Add(1) == 1 {
			close(started)
		}
		<-release
		return backend.ActiveStateSnapshot{ActiveAccount: "one@example.com"}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_ = cache.snapshot(false, build)
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("singleflight builds=%d, want 1", builds.Load())
	}
}

func TestActiveStateSnapshotCacheInvalidate(t *testing.T) {
	var cache activeStateCache
	var builds atomic.Int32
	build := func() backend.ActiveStateSnapshot {
		builds.Add(1)
		return backend.ActiveStateSnapshot{ActiveAccount: "one@example.com"}
	}
	_ = cache.snapshot(false, build)
	cache.invalidate()
	after := cache.snapshot(false, build)
	if builds.Load() != 2 || after.Cache.Generation != 1 {
		t.Fatalf("invalidate did not force rebuild: builds=%d stats=%+v", builds.Load(), after.Cache)
	}
}
