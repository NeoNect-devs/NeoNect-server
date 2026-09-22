package api

import (
	"NeoNect/internal/config"
	"hash/maphash"
	"os"
	"strconv"
	"sync"
	"time"
)

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsedValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsedValue
}

// RateWindow implements a strictly bounded sliding window log using a ring buffer.
type RateWindow struct {
	timestamps []time.Time
	head       int
	count      int
}

// Prune removes timestamps that are >= window old.
// Exact active condition: now.Sub(timestamp) < window.
func (rw *RateWindow) Prune(now time.Time, window time.Duration) {
	for rw.count > 0 {
		ts := rw.timestamps[rw.head]
		if now.Sub(ts) < window {
			break
		}
		rw.head = (rw.head + 1) % len(rw.timestamps)
		rw.count--
	}
}

// Record attempts to add a new timestamp.
// Ensures strict memory capacity and returns false if limit is reached.
func (rw *RateWindow) Record(now time.Time, limit int) bool {
	if rw.count >= limit {
		return false
	}

	// Small-buffer bounded allocation up to Limit
	if len(rw.timestamps) == 0 {
		newCap := 4
		if limit < 4 {
			newCap = limit
		}
		rw.timestamps = make([]time.Time, newCap)
		rw.head = 0
	} else if rw.count == len(rw.timestamps) && len(rw.timestamps) < limit {
		newCap := len(rw.timestamps) * 2
		if newCap > limit {
			newCap = limit
		}
		newTs := make([]time.Time, newCap)
		for i := 0; i < rw.count; i++ {
			newTs[i] = rw.timestamps[(rw.head+i)%len(rw.timestamps)]
		}
		rw.timestamps = newTs
		rw.head = 0
	}

	rw.timestamps[(rw.head+rw.count)%len(rw.timestamps)] = now
	rw.count++
	return true
}

// TimeWindowLimiter shards time-based rate limit entries to prevent lock contention
// and strictly bounds map growth per shard.
type TimeWindowLimiter[K comparable] struct {
	shards  []*windowShard[K]
	seed    maphash.Seed
	hasher  func(K, maphash.Seed) uint64
	maxKeys int
}

type windowShard[K comparable] struct {
	mu      sync.Mutex
	entries map[K]*RateWindow
}

func NewTimeWindowLimiter[K comparable](numShards, maxKeys int, hasher func(K, maphash.Seed) uint64) *TimeWindowLimiter[K] {
	limiter := &TimeWindowLimiter[K]{
		shards:  make([]*windowShard[K], numShards),
		seed:    maphash.MakeSeed(),
		hasher:  hasher,
		maxKeys: maxKeys,
	}
	for i := 0; i < numShards; i++ {
		limiter.shards[i] = &windowShard[K]{
			entries: make(map[K]*RateWindow),
		}
	}
	return limiter
}

func (l *TimeWindowLimiter[K]) Allow(key K, limit int, now time.Time, window time.Duration) bool {
	hash := l.hasher(key, l.seed)
	shardIdx := hash % uint64(len(l.shards))
	shard := l.shards[shardIdx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	rw, exists := shard.entries[key]
	if !exists {
		// New identity: check strict memory bounds
		if len(shard.entries) >= l.maxKeys {
			// Sweep fully expired entries in this shard
			for k, win := range shard.entries {
				win.Prune(now, window)
				if win.count == 0 {
					delete(shard.entries, k)
				}
			}
			// Fail-closed admission if still full
			if len(shard.entries) >= l.maxKeys {
				return false
			}
		}
		rw = &RateWindow{}
		shard.entries[key] = rw
	} else {
		rw.Prune(now, window)
	}

	return rw.Record(now, limit)
}

// GaugeLimiter tracks active concurrent states (e.g. TCP connections).
// It never evicts active counts.
type GaugeLimiter[K comparable] struct {
	shards []*gaugeShard[K]
	seed   maphash.Seed
	hasher func(K, maphash.Seed) uint64
}

type gaugeShard[K comparable] struct {
	mu      sync.Mutex
	entries map[K]int
}

func NewGaugeLimiter[K comparable](numShards int, hasher func(K, maphash.Seed) uint64) *GaugeLimiter[K] {
	limiter := &GaugeLimiter[K]{
		shards: make([]*gaugeShard[K], numShards),
		seed:   maphash.MakeSeed(),
		hasher: hasher,
	}
	for i := 0; i < numShards; i++ {
		limiter.shards[i] = &gaugeShard[K]{
			entries: make(map[K]int),
		}
	}
	return limiter
}

func (l *GaugeLimiter[K]) Allow(key K, limit int) bool {
	hash := l.hasher(key, l.seed)
	shardIdx := hash % uint64(len(l.shards))
	shard := l.shards[shardIdx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	count := shard.entries[key]
	if count >= limit {
		return false
	}
	shard.entries[key] = count + 1
	return true
}

func (l *GaugeLimiter[K]) Release(key K) {
	hash := l.hasher(key, l.seed)
	shardIdx := hash % uint64(len(l.shards))
	shard := l.shards[shardIdx]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	count := shard.entries[key]
	if count > 0 {
		count--
		if count == 0 {
			delete(shard.entries, key)
		} else {
			shard.entries[key] = count
		}
	}
}

// Hashers
func stringHasher(key string, seed maphash.Seed) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	h.WriteString(key)
	return h.Sum64()
}

func int64Hasher(key int64, seed maphash.Seed) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	b := [8]byte{
		byte(key), byte(key >> 8), byte(key >> 16), byte(key >> 24),
		byte(key >> 32), byte(key >> 40), byte(key >> 48), byte(key >> 56),
	}
	h.Write(b[:])
	return h.Sum64()
}

// RequestRateLimiter is the main entry point combining the independent limiters.
type RequestRateLimiter struct {
	mu          sync.RWMutex
	connections *GaugeLimiter[string]
	ipReq       *TimeWindowLimiter[string]
	ipAuth      *TimeWindowLimiter[string]
	usrMsg      *TimeWindowLimiter[int64]
	usrDisc     *TimeWindowLimiter[int64]

	maxConnPerIP       int
	maxReqPerSecIP     int
	maxMsgPerSec       int
	maxDiscoveryPerSec int
	maxAuthPerSecIP    int

	rateWindow time.Duration
	nowFunc    func() time.Time
}

func NewRateLimiter() *RequestRateLimiter {
	maxConn := getEnvInt("NEONECT_MAX_CONN_PER_IP", config.RateLimitMaxConnPerIP)
	maxReq := getEnvInt("NEONECT_MAX_REQ_PER_SEC_IP", config.RateLimitMaxReqPerSecIP)
	maxMsg := getEnvInt("NEONECT_MAX_MSG_PER_SEC", config.RateLimitMaxMsgPerSec)
	maxDisc := getEnvInt("NEONECT_MAX_DISCOVERY_PER_SEC", config.RateLimitMaxDiscoveryPerSec)
	maxAuth := 5

	return &RequestRateLimiter{
		connections: NewGaugeLimiter(64, stringHasher),
		ipReq:       NewTimeWindowLimiter(64, 218, stringHasher),
		ipAuth:      NewTimeWindowLimiter(64, 484, stringHasher),
		usrMsg:      NewTimeWindowLimiter(64, 62, int64Hasher),
		usrDisc:     NewTimeWindowLimiter(64, 200, int64Hasher),

		maxConnPerIP:       maxConn,
		maxReqPerSecIP:     maxReq,
		maxMsgPerSec:       maxMsg,
		maxDiscoveryPerSec: maxDisc,
		maxAuthPerSecIP:    maxAuth,

		rateWindow: time.Second,
		nowFunc:    time.Now,
	}
}

func (rl *RequestRateLimiter) Stop() {
	// The new design requires no background goroutine for cleanup.
	// Expired entries are swept lazily on insert pressure.
}

func (rl *RequestRateLimiter) SetLimits(maxConnections, maxMessages int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maxConnPerIP = maxConnections
	rl.maxMsgPerSec = maxMessages
}

func (rl *RequestRateLimiter) SetIPLimits(maxReq, maxAuth int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maxReqPerSecIP = maxReq
	rl.maxAuthPerSecIP = maxAuth
}

func (rl *RequestRateLimiter) AllowConnection(ipAddress string) bool {
	rl.mu.RLock()
	limit := rl.maxConnPerIP
	rl.mu.RUnlock()
	return rl.connections.Allow(ipAddress, limit)
}

func (rl *RequestRateLimiter) ReleaseConnection(ipAddress string) {
	rl.connections.Release(ipAddress)
}

func (rl *RequestRateLimiter) AllowIPRequest(ipAddress string) bool {
	rl.mu.RLock()
	limit := rl.maxReqPerSecIP
	window := rl.rateWindow
	now := rl.nowFunc()
	rl.mu.RUnlock()
	return rl.ipReq.Allow(ipAddress, limit, now, window)
}

func (rl *RequestRateLimiter) AllowMessage(userID int64) bool {
	rl.mu.RLock()
	limit := rl.maxMsgPerSec
	window := rl.rateWindow
	now := rl.nowFunc()
	rl.mu.RUnlock()
	return rl.usrMsg.Allow(userID, limit, now, window)
}

func (rl *RequestRateLimiter) AllowDiscovery(userID int64) bool {
	rl.mu.RLock()
	limit := rl.maxDiscoveryPerSec
	if limit == 0 {
		limit = 10
	}
	window := rl.rateWindow
	now := rl.nowFunc()
	rl.mu.RUnlock()
	return rl.usrDisc.Allow(userID, limit, now, window)
}

func (rl *RequestRateLimiter) AllowAuthAttempt(ipAddress string) bool {
	rl.mu.RLock()
	limit := rl.maxAuthPerSecIP
	window := rl.rateWindow
	now := rl.nowFunc()
	rl.mu.RUnlock()
	return rl.ipAuth.Allow(ipAddress, limit, now, window)
}
