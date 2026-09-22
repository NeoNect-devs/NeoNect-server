package api

import (
	"sync"
	"testing"
	"time"
)

func TestRateWindow_ExactBoundary(t *testing.T) {
	rw := &RateWindow{}

	now := time.Now()
	// T = 10.000
	t0 := now
	if !rw.Record(t0, 1) {
		t.Fatal("Failed to record first")
	}

	// T = 10.999
	rw.Prune(t0.Add(999*time.Millisecond), time.Second)
	if rw.Record(t0.Add(999*time.Millisecond), 1) {
		t.Fatal("Should reject at 999ms")
	}

	// T = 11.000
	rw.Prune(t0.Add(1000*time.Millisecond), time.Second)
	if !rw.Record(t0.Add(1000*time.Millisecond), 1) {
		t.Fatal("Should accept at exactly 1000ms")
	}

	// T = 11.001
	rw.Prune(t0.Add(1001*time.Millisecond), time.Second)
	if rw.Record(t0.Add(1001*time.Millisecond), 1) {
		t.Fatal("Should reject at 1001ms because 1000ms is still active")
	}
}

func TestRateWindow_RingBufferCapacity(t *testing.T) {
	rw := &RateWindow{}
	limit := 5

	now := time.Now()
	for i := 0; i < limit; i++ {
		if !rw.Record(now, limit) {
			t.Fatalf("Failed at %d", i)
		}
	}

	if rw.Record(now, limit) {
		t.Fatal("Allowed beyond limit")
	}

	if len(rw.timestamps) != limit {
		t.Fatalf("Expected len %d, got %d", limit, len(rw.timestamps))
	}
	if cap(rw.timestamps) != limit {
		t.Fatalf("Expected cap %d, got %d", limit, cap(rw.timestamps))
	}
}

func TestRateWindow_SparseIdentity(t *testing.T) {
	rw := &RateWindow{}
	rw.Record(time.Now(), 50)

	if cap(rw.timestamps) > 4 {
		t.Fatalf("Expected small allocation for sparse identity, got cap %d", cap(rw.timestamps))
	}
}

func TestTimeWindowLimiter_FullShard(t *testing.T) {
	limiter := NewTimeWindowLimiter[string](1, 2, stringHasher)
	now := time.Now()

	limiter.Allow("ip1", 5, now, time.Second)
	limiter.Allow("ip2", 5, now, time.Second)

	if limiter.Allow("ip3", 5, now, time.Second) {
		t.Fatal("Should reject admission when full of active entries")
	}

	if !limiter.Allow("ip1", 5, now, time.Second) {
		t.Fatal("Existing identity should still be processed")
	}

	// advance time to expire ip1 and ip2
	now = now.Add(2 * time.Second)

	if !limiter.Allow("ip3", 5, now, time.Second) {
		t.Fatal("Should allow after old entries expired")
	}
}

func TestGaugeLimiter_IsolationAndConcurrent(t *testing.T) {
	gauge := NewGaugeLimiter[string](4, stringHasher)

	var wg sync.WaitGroup
	ip := "127.0.0.1"

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if gauge.Allow(ip, 5000) {
				gauge.Release(ip)
			}
		}()
	}
	wg.Wait()

	shard := gauge.shards[stringHasher(ip, gauge.seed)%uint64(len(gauge.shards))]
	shard.mu.Lock()
	count, exists := shard.entries[ip]
	shard.mu.Unlock()

	if exists && count != 0 {
		t.Fatalf("Expected 0 connections, got %d", count)
	}
}

func TestRateLimiter_Integration(t *testing.T) {
	rl := NewRateLimiter()

	// Inject clock
	currentTime := time.Now()
	rl.nowFunc = func() time.Time { return currentTime }

	// Test connection gauge
	if !rl.AllowConnection("10.0.0.1") {
		t.Fatal("Failed first connection")
	}
	for i := 1; i < rl.maxConnPerIP; i++ {
		rl.AllowConnection("10.0.0.1")
	}
	if rl.AllowConnection("10.0.0.1") {
		t.Fatal("Allowed beyond connection limit")
	}
	rl.ReleaseConnection("10.0.0.1")
	if !rl.AllowConnection("10.0.0.1") {
		t.Fatal("Failed connection after release")
	}

	// Test Auth limits
	ip := "10.0.0.2"
	for i := 0; i < rl.maxAuthPerSecIP; i++ {
		if !rl.AllowAuthAttempt(ip) {
			t.Fatalf("Failed auth limit at %d", i)
		}
	}
	if rl.AllowAuthAttempt(ip) {
		t.Fatal("Allowed beyond auth limit")
	}

	// Advance clock
	currentTime = currentTime.Add(1001 * time.Millisecond)
	if !rl.AllowAuthAttempt(ip) {
		t.Fatal("Failed auth limit after 1 second expiration")
	}
}
