package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 是 P0 简易指标收集器，后续可替换为 Prometheus。
type Metrics struct {
	encryptOps       atomic.Int64
	decryptOps       atomic.Int64
	encryptErrors    atomic.Int64
	decryptErrors    atomic.Int64
	dekCacheHits     atomic.Int64
	dekCacheMisses   atomic.Int64
	nonceAllocations atomic.Int64
	nonceExhaustions atomic.Int64
	tpmUnseals       atomic.Int64
	tpmUnsealErrors  atomic.Int64

	encryptLatencySum atomic.Int64 // 纳秒
	encryptLatencyCnt atomic.Int64
	decryptLatencySum atomic.Int64
	decryptLatencyCnt atomic.Int64

	mu     sync.Mutex
	rates  map[string]*rateBucket
}

type rateBucket struct {
	count    int64
	window   time.Duration
	lastTime time.Time
}

var globalMetrics = &Metrics{rates: make(map[string]*rateBucket)}

func MetricsInstance() *Metrics { return globalMetrics }

func (m *Metrics) IncEncryptOp()        { m.encryptOps.Add(1) }
func (m *Metrics) IncDecryptOp()        { m.decryptOps.Add(1) }
func (m *Metrics) IncEncryptError()     { m.encryptErrors.Add(1) }
func (m *Metrics) IncDecryptError()     { m.decryptErrors.Add(1) }
func (m *Metrics) IncDEKCacheHit()      { m.dekCacheHits.Add(1) }
func (m *Metrics) IncDEKCacheMiss()     { m.dekCacheMisses.Add(1) }
func (m *Metrics) IncNonceAllocation()  { m.nonceAllocations.Add(1) }
func (m *Metrics) IncNonceExhaustion()  { m.nonceExhaustions.Add(1) }
func (m *Metrics) IncTPMUnseal()        { m.tpmUnseals.Add(1) }
func (m *Metrics) IncTPMUnsealError()   { m.tpmUnsealErrors.Add(1) }

func (m *Metrics) RecordEncryptLatency(d time.Duration) {
	m.encryptLatencySum.Add(int64(d))
	m.encryptLatencyCnt.Add(1)
}

func (m *Metrics) RecordDecryptLatency(d time.Duration) {
	m.decryptLatencySum.Add(int64(d))
	m.decryptLatencyCnt.Add(1)
}

// Snapshot 返回当前指标快照。
type Snapshot struct {
	EncryptOps       int64 `json:"encrypt_ops"`
	DecryptOps       int64 `json:"decrypt_ops"`
	EncryptErrors    int64 `json:"encrypt_errors"`
	DecryptErrors    int64 `json:"decrypt_errors"`
	DEKCacheHits     int64 `json:"dek_cache_hits"`
	DEKCacheMisses   int64 `json:"dek_cache_misses"`
	NonceAllocations int64 `json:"nonce_allocations"`
	NonceExhaustions int64 `json:"nonce_exhaustions"`
	TPMUnseals       int64 `json:"tpm_unseals"`
	TPMUnsealErrors  int64 `json:"tpm_unseal_errors"`
}

func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		EncryptOps:       m.encryptOps.Load(),
		DecryptOps:       m.decryptOps.Load(),
		EncryptErrors:    m.encryptErrors.Load(),
		DecryptErrors:    m.decryptErrors.Load(),
		DEKCacheHits:     m.dekCacheHits.Load(),
		DEKCacheMisses:   m.dekCacheMisses.Load(),
		NonceAllocations: m.nonceAllocations.Load(),
		NonceExhaustions: m.nonceExhaustions.Load(),
		TPMUnseals:       m.tpmUnseals.Load(),
		TPMUnsealErrors:  m.tpmUnsealErrors.Load(),
	}
}
