// Package keyresolver 实现数据面密钥解析器（第 9.4 节）。
//
// Resolver 是数据面与密钥面之间的桥梁，负责 DEK lease 签发和 nonce 分配。
// 它通过 DEK 缓存减少 TPM 解封次数，通过 singleflight 合并并发解封请求（HA-12），
// 通过 nonce lease 实现批量 nonce 分配，降低分布式协调开销。
package keyresolver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/HangzeGao/trae/key-vault/internal/config"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/datakey"
	"github.com/HangzeGao/trae/key-vault/internal/crypto/nonce"
	apperrors "github.com/HangzeGao/trae/key-vault/internal/errors"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
	"github.com/HangzeGao/trae/key-vault/internal/repository/postgres"
	"github.com/HangzeGao/trae/key-vault/internal/tpm/provider"
)

// nonceLeaseTTL 是 nonce 租约的默认有效期。
// nonce 租约过期后会自动续租，此值仅用于清理过期租约。
const nonceLeaseTTL = 1 * time.Hour

// dekCleanupInterval 是 DEK 缓存清理 goroutine 的执行间隔。
// 定期扫描追踪表，清零并移除已过期的 DEK 明文。
const dekCleanupInterval = 30 * time.Second

// prefetchBufferSize 是预取请求 channel 的缓冲区大小。
const prefetchBufferSize = 100

// DEKLease 是 DEK 租约，由 Resolver 签发给数据面调用方。
//
// 调用方在使用完 DEK 后应自行清零（遍历赋零），避免明文长期驻留内存。
type DEKLease struct {
	KID       string    // key_id/version 标识
	DEK       []byte    // DEK 明文（调用方使用后应清零）
	Algorithm string    // 算法套件（AES_256_GCM / SM4_128_GCM）
	ExpiresAt time.Time // 过期时间
}

// trackedDEK 追踪缓存中的 DEK，用于过期后清零明文。
type trackedDEK struct {
	dek       []byte
	expiresAt time.Time
}

// nonceCounter 是单节点 nonce 区间计数器，为 nonce.LeaseManager 提供续租函数。
//
// 每个 KID 维护一个单调递增的计数器，续租时分配 [lastEnd+1, lastEnd+leaseSize] 区间。
// 多节点部署需替换为基于数据库序列或分布式协调的实现，以避免 nonce 区间重叠。
type nonceCounter struct {
	mu        sync.Mutex
	counters  map[string]uint64
	leaseSize uint64
	ttl       time.Duration
}

// newNonceCounter 创建一个 nonce 区间计数器。
func newNonceCounter(leaseSize uint64, ttl time.Duration) *nonceCounter {
	return &nonceCounter{
		counters:  make(map[string]uint64),
		leaseSize: leaseSize,
		ttl:       ttl,
	}
}

// renew 为指定 KID 分配新的 nonce 区间。
// 返回 [start, end] 闭区间和过期时间。
func (c *nonceCounter) renew(kid string) (start, end uint64, expiresAt time.Time, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	lastEnd := c.counters[kid]
	start = lastEnd + 1
	end = start + c.leaseSize - 1
	c.counters[kid] = end
	expiresAt = time.Now().Add(c.ttl)
	return
}

// Resolver 是密钥解析器，负责 DEK lease 签发和 nonce 分配（第 9.4 节）。
// 它是数据面与密钥面之间的桥梁。
type Resolver struct {
	db       *sql.DB
	keyRepo  *postgres.KeyRepository
	tpm      provider.Provider
	dekCache *datakey.Cache
	nonceMgr *nonce.SingleflightLeaseManager
	log      *observability.Logger
	metrics  *observability.Metrics
	config   config.CryptoConfig
	// sfGroup 合并相同 KID 的 unseal 请求（HA-12），避免高并发下重复解封。
	sfGroup  singleflight.Group
	degraded atomic.Bool

	// trackedDEKs 追踪缓存中的 DEK，用于过期后清零明文。
	trackedDEKs sync.Map // kid -> *trackedDEK
	// algorithms 缓存 KID 对应的算法，避免 cache hit 时重复查库。
	algorithms sync.Map // kid -> string

	// nonceCounter 为 nonceMgr 提供区间分配。
	nonceCounter *nonceCounter

	// prefetchCh 是异步预取请求 channel。
	prefetchCh chan prefetchRequest

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewResolver 创建一个密钥解析器实例。
//
// 内部会创建 DEK 缓存、nonce 租约管理器，并启动后台 DEK 清理 goroutine。
// tpm 应已调用 Init 完成初始化。调用方负责在服务关闭时调用 Close 释放资源。
func NewResolver(db *sql.DB, keyRepo *postgres.KeyRepository, tpm provider.Provider, cfg config.CryptoConfig, log *observability.Logger) *Resolver {
	ctx, cancel := context.WithCancel(context.Background())

	r := &Resolver{
		db:           db,
		keyRepo:      keyRepo,
		tpm:          tpm,
		dekCache:     datakey.NewCache(),
		log:          log,
		metrics:      observability.MetricsInstance(),
		config:       cfg,
		prefetchCh:   make(chan prefetchRequest, prefetchBufferSize),
		ctx:          ctx,
		cancel:       cancel,
		nonceCounter: newNonceCounter(cfg.NonceLeaseSize, nonceLeaseTTL),
	}

	// 创建 nonce 租约管理器。
	// 使用前向声明解决 PrefetchCallback 与 SingleflightLeaseManager 的循环依赖：
	// PrefetchCallback 需要调用 sfMgr.Renew，而 sfMgr 在 LeaseManager 创建之后才构造。
	var sfMgr *nonce.SingleflightLeaseManager
	leaseMgr := nonce.NewLeaseManager(
		r.nonceCounter.renew,
		func(kid string) {
			if sfMgr != nil {
				// 异步触发续租，使用 singleflight 合并并发请求
				go sfMgr.Renew(kid)
			}
		},
	)
	sfMgr = nonce.NewSingleflightLeaseManager(leaseMgr)
	r.nonceMgr = sfMgr

	// 启动后台 DEK 清理 goroutine，定期清零过期 DEK 明文
	r.wg.Add(1)
	go r.cleanupExpiredDEKs()

	return r
}

// IssueDEKLease 签发 DEK lease（第 9.4 节加密流程）。
//
// 流程：
//  1. 先查 DEK cache，命中则返回（IncDEKCacheHit）
//  2. cache miss 时记录 IncDEKCacheMiss
//  3. 若 resolver 处于降级状态，快速失败返回 TPMUnavailable
//  4. 验证租户访问并获取密钥算法
//  5. 使用 singleflight 合并相同 KID 的 unseal 请求（HA-12）
//  6. 在 singleflight 内：
//     a. 从 DB 查 KeyVersion，获取 WrappedDEK
//     b. 调用 TPM UnsealDEK 获取 DEK 明文（受 ResolverTimeout 超时控制）
//     c. 存入 cache（带 TTL）
//  7. 返回 DEK lease（DEK 为副本，调用方可安全清零）
//
// TPM 致命错误时设置 degraded 状态，后续请求快速失败，不阻塞热路径。
func (r *Resolver) IssueDEKLease(ctx context.Context, tenantID, keyID string, version int) (*DEKLease, error) {
	kid := formatKID(keyID, version)

	// 1. 先查 DEK cache（热路径）
	if lease, ok := r.dekCache.Get(kid); ok {
		r.metrics.IncDEKCacheHit()
		return &DEKLease{
			KID:       kid,
			DEK:       copyBytes(lease.DEK),
			Algorithm: r.lookupAlgorithm(kid),
			ExpiresAt: lease.ExpiresAt,
		}, nil
	}

	// 2. cache miss
	r.metrics.IncDEKCacheMiss()

	// 3. 降级时快速失败，不阻塞热路径
	if r.degraded.Load() {
		return nil, apperrors.TPMUnavailable(fmt.Errorf("resolver is in degraded mode"))
	}

	// 4. 验证租户访问并获取密钥算法（租户隔离，HA-11）
	k, err := r.keyRepo.GetKey(ctx, r.db, tenantID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.KeyNotFound()
		}
		return nil, apperrors.Internal(err)
	}
	algorithm := string(k.Algorithm)

	// 5. singleflight 合并相同 KID 的 unseal 请求（HA-12）
	v, err, _ := r.sfGroup.Do(kid, func() (interface{}, error) {
		return r.unsealAndCache(keyID, version, kid, algorithm)
	})
	if err != nil {
		return nil, err
	}

	dek := v.([]byte)
	return &DEKLease{
		KID:       kid,
		DEK:       copyBytes(dek),
		Algorithm: algorithm,
		ExpiresAt: time.Now().Add(r.config.DEKCacheTTL),
	}, nil
}

// unsealAndCache 从 DB 查询 WrappedDEK，调用 TPM 解封，并存入缓存。
//
// 此方法在 singleflight 内执行，使用独立的超时 context（ResolverTimeout），
// 不受单个调用方 context 取消的影响，保证其他等待者能获得结果。
// TPM 致命错误时设置 degraded 状态。
func (r *Resolver) unsealAndCache(keyID string, version int, kid, algorithm string) ([]byte, error) {
	// 创建带超时的 context（跨平面调用受 ResolverTimeout 控制）
	ctx, cancel := context.WithTimeout(context.Background(), r.config.ResolverTimeout)
	defer cancel()

	// 从 DB 查 KeyVersion，获取 WrappedDEK
	kv, err := r.keyRepo.GetKeyVersion(ctx, r.db, keyID, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperrors.KeyNotFound()
		}
		return nil, apperrors.Internal(err)
	}

	// 调用 TPM UnsealDEK 获取 DEK 明文
	dek, err := r.tpm.UnsealDEK(ctx, kv.WrappedDEK)
	if err != nil {
		// TPM 致命错误时设置降级状态，后续请求快速失败
		if provider.IsFatal(err) {
			r.degraded.Store(true)
			r.log.Error("TPM fatal error, entering degraded mode",
				"error", err,
				"kid", kid,
			)
		}
		return nil, apperrors.TPMUnavailable(err)
	}

	// 存入 cache（带 TTL）
	ttl := r.config.DEKCacheTTL
	r.dekCache.Set(kid, dek, ttl)

	// 追踪 DEK 用于过期后清零明文
	r.trackedDEKs.Store(kid, &trackedDEK{
		dek:       dek,
		expiresAt: time.Now().Add(ttl),
	})

	// 缓存算法，避免 cache hit 时重复查库
	r.algorithms.Store(kid, algorithm)

	return dek, nil
}

// AllocateNonce 为指定 KID 分配一个 nonce。
//
// 委托给 nonce.SingleflightLeaseManager，并记录 IncNonceAllocation 指标。
// nonce 耗尽时记录 IncNonceExhaustion 并返回 NonceExhausted 错误。
func (r *Resolver) AllocateNonce(ctx context.Context, kid string) (uint64, error) {
	n, err := r.nonceMgr.Allocate(kid)
	if err != nil {
		r.metrics.IncNonceExhaustion()
		return 0, err
	}
	r.metrics.IncNonceAllocation()
	return n, nil
}

// RenewNonceLease 续租 nonce lease。
//
// 委托给 nonce.SingleflightLeaseManager，使用 singleflight 合并相同 KID 的续租请求。
func (r *Resolver) RenewNonceLease(ctx context.Context, kid string) error {
	return r.nonceMgr.Renew(kid)
}

// InvalidateCache 失效指定 KID 的 DEK cache（密钥轮转后调用）。
//
// 同时清零对应的 DEK 明文，避免内存中残留过期密钥材料。
func (r *Resolver) InvalidateCache(kid string) {
	if v, ok := r.trackedDEKs.LoadAndDelete(kid); ok {
		td := v.(*trackedDEK)
		zeroBytes(td.dek)
	}
	r.dekCache.Delete(kid)
	r.algorithms.Delete(kid)
}

// IsDegraded 返回 resolver 是否处于降级状态。
//
// 降级状态下，DEK cache miss 请求会快速失败返回 TPMUnavailable。
func (r *Resolver) IsDegraded() bool {
	return r.degraded.Load()
}

// SetDegraded 设置降级状态（健康检查调用）。
//
// 设为 true 后，后续 DEK cache miss 请求快速失败；
// 设为 false 后，恢复正常解封流程。
func (r *Resolver) SetDegraded(degraded bool) {
	r.degraded.Store(degraded)
}

// Close 清理资源。
//
// 停止后台清理 goroutine，清零所有缓存的 DEK 明文。
// 调用后 Resolver 不应再被使用。
func (r *Resolver) Close() error {
	r.cancel()
	r.wg.Wait()

	// 清零所有缓存的 DEK 明文
	r.trackedDEKs.Range(func(key, value interface{}) bool {
		td := value.(*trackedDEK)
		zeroBytes(td.dek)
		r.trackedDEKs.Delete(key)
		return true
	})

	return nil
}

// cleanupExpiredDEKs 后台定期清理过期的 DEK，清零明文并从缓存中移除。
//
// 每 dekCleanupInterval 执行一次扫描，对已过期的 trackedDEK：
//   - 清零 DEK 明文（防止内存残留）
//   - 从 trackedDEKs、dekCache、algorithms 中移除
func (r *Resolver) cleanupExpiredDEKs() {
	defer r.wg.Done()

	ticker := time.NewTicker(dekCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.doCleanup()
		}
	}
}

// doCleanup 执行一次过期 DEK 清理操作。
func (r *Resolver) doCleanup() {
	now := time.Now()
	r.trackedDEKs.Range(func(key, value interface{}) bool {
		kid := key.(string)
		td := value.(*trackedDEK)
		if now.After(td.expiresAt) {
			zeroBytes(td.dek)
			r.trackedDEKs.Delete(kid)
			r.dekCache.Delete(kid)
			r.algorithms.Delete(kid)
		}
		return true
	})
}

// lookupAlgorithm 查找 KID 对应的算法。
// 若未找到则返回配置的默认算法套件。
func (r *Resolver) lookupAlgorithm(kid string) string {
	if v, ok := r.algorithms.Load(kid); ok {
		return v.(string)
	}
	return r.config.DefaultSuite
}

// formatKID 构造 KID 标识（key_id/v{version}）。
func formatKID(keyID string, version int) string {
	return fmt.Sprintf("%s/v%d", keyID, version)
}

// copyBytes 返回 b 的副本，使调用方与缓存持有独立的底层数组。
//
// 这样调用方清零自己的 DEK 副本时不会影响缓存中的 DEK。
func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// zeroBytes 将 b 清零，使用 runtime.KeepAlive 防止编译器优化掉写入。
//
// 用于在 DEK lease 过期或失效时清除内存中的明文密钥材料。
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
