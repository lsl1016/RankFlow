package observability

import (
	"sync/atomic"
	"time"
)

// Metrics holds lightweight in-process counters surfaced on the rank detail
// page. It is intentionally simple (cumulative counters + average QPS) rather
// than a full Prometheus integration, which is a phase-two concern.
type Metrics struct {
	writeCount atomic.Int64
	readCount  atomic.Int64
	cacheHit   atomic.Int64
	cacheMiss  atomic.Int64
	startedAt  time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now()}
}

func (m *Metrics) IncWrite()    { m.writeCount.Add(1) }
func (m *Metrics) IncRead()     { m.readCount.Add(1) }
func (m *Metrics) IncCacheHit() { m.cacheHit.Add(1) }
func (m *Metrics) IncCacheMiss(){ m.cacheMiss.Add(1) }

type Snapshot struct {
	WriteCount   int64   `json:"writeCount"`
	ReadCount    int64   `json:"readCount"`
	WriteQPS     float64 `json:"writeQps"`
	ReadQPS      float64 `json:"readQps"`
	CacheHitRate float64 `json:"cacheHitRate"`
	UptimeSec    int64   `json:"uptimeSec"`
}

func (m *Metrics) Snapshot() Snapshot {
	elapsed := time.Since(m.startedAt).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	w := m.writeCount.Load()
	r := m.readCount.Load()
	hit := m.cacheHit.Load()
	miss := m.cacheMiss.Load()
	var hitRate float64
	if hit+miss > 0 {
		hitRate = float64(hit) / float64(hit+miss)
	}
	return Snapshot{
		WriteCount:   w,
		ReadCount:    r,
		WriteQPS:     float64(w) / elapsed,
		ReadQPS:      float64(r) / elapsed,
		CacheHitRate: hitRate,
		UptimeSec:    int64(elapsed),
	}
}
