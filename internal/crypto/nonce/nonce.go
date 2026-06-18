// Package nonce 提供 nonce 分配器，基于 per-KID lease 区间分配单调递增的 nonce。
//
// 设计要点（第 8 章）：
//   - 每个 KID 持有一个 Lease，包含 [Start, End] 闭区间
//   - Allocate 从当前 lease 分配下一个 nonce 编号，递增 Next 游标
//   - 使用量达 70% 时通过 callback 触发异步预取续租
//   - 使用量达 90% 时进入 critical 水位
//   - lease 耗尽或过期时同步续租，续租失败返回 NonceExhausted
//   - NonceToBytes 将编号编码为 12 字节 big-endian nonce（GCM 标准）
//
// SingleflightLeaseManager 对相同 KID 的续租请求做 singleflight 合并（HA-12），
// 避免高并发下重复向后端申请 lease。
package nonce

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/errors"
	"golang.org/x/sync/singleflight"
)

// NonceSize 是 GCM nonce 的字节长度。
const NonceSize = 12

// 使用量阈值。
const (
	prefetchThreshold = 0.7 // 触发异步预取的使用量阈值
	criticalThreshold = 0.9 // 进入 critical 水位的使用量阈值
)

// Lease 表示一个 KID 的 nonce 租约区间。
type Lease struct {
	KID       string    // 关联的 key ID
	Start     uint64    // 区间起始（含）
	End       uint64    // 区间结束（含）
	Next      uint64    // 下一个待分配的 nonce 编号
	ExpiresAt time.Time // 租约过期时间
	// prefetched 标记当前 lease 是否已触发预取，避免重复触发。
	prefetched bool
	// critical 标记当前 lease 是否已进入 critical 水位。
	critical bool
}

// Allocator 是 nonce 分配器接口。
type Allocator interface {
	// Allocate 为指定 KID 分配下一个 nonce 编号。
	// 若 lease 耗尽且续租失败，返回 NonceExhausted 错误。
	Allocate(kid string) (nonce uint64, err error)
	// Renew 为指定 KID 同步续租 lease。
	Renew(kid string) error
}

// RenewFunc 从后端获取新的 nonce lease 区间。
// 返回新区间的 [start, end] 和过期时间。
type RenewFunc func(kid string) (start, end uint64, expiresAt time.Time, err error)

// PrefetchCallback 是预取回调，在使用量达到阈值时被异步调用。
// 通常实现为异步调用 Renew。
type PrefetchCallback func(kid string)

// LeaseManager 基于 per-KID lease 的 nonce 分配器实现。
type LeaseManager struct {
	mu         sync.Mutex
	leases     map[string]*Lease
	renew      RenewFunc
	prefetchCB PrefetchCallback
}

// NewLeaseManager 创建 LeaseManager。
// renew 是续租函数，prefetchCB 是预取回调（可为 nil）。
func NewLeaseManager(renew RenewFunc, prefetchCB PrefetchCallback) *LeaseManager {
	return &LeaseManager{
		leases:     make(map[string]*Lease),
		renew:      renew,
		prefetchCB: prefetchCB,
	}
}

// Allocate 为指定 KID 分配下一个 nonce 编号。
//
// 流程：
//  1. 若无 lease、lease 耗尽或过期，同步续租；续租失败返回 NonceExhausted
//  2. 从 lease 分配下一个 nonce，递增 Next
//  3. 使用量达 70% 时触发异步预取（每个 lease 仅触发一次）
//  4. 使用量达 90% 时标记 critical 水位
func (m *LeaseManager) Allocate(kid string) (uint64, error) {
	// 检查是否需要同步续租
	if m.needRenewLocked(kid) {
		if err := m.Renew(kid); err != nil {
			return 0, err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	lease := m.leases[kid]
	// 续租后仍可能为 nil（理论上不会，防御性检查）
	if lease == nil || lease.Next > lease.End {
		return 0, errors.NonceExhausted()
	}

	nonce := lease.Next
	lease.Next++

	// 计算使用量并触发预取/标记 critical
	m.checkThresholdsLocked(lease)

	return nonce, nil
}

// needRenewLocked 检查指定 KID 是否需要续租。
// 调用此方法会获取并释放锁，不持有锁返回。
func (m *LeaseManager) needRenewLocked(kid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, ok := m.leases[kid]
	if !ok {
		return true
	}
	if lease.Next > lease.End {
		return true
	}
	if time.Now().After(lease.ExpiresAt) {
		return true
	}
	return false
}

// checkThresholdsLocked 检查使用量阈值并触发预取/标记 critical。
// 调用方必须持有 m.mu。
func (m *LeaseManager) checkThresholdsLocked(lease *Lease) {
	total := lease.End - lease.Start + 1
	if total == 0 {
		return
	}
	used := lease.Next - lease.Start
	usage := float64(used) / float64(total)

	// 70% 触发异步预取（每个 lease 仅触发一次）
	if usage >= prefetchThreshold && !lease.prefetched {
		lease.prefetched = true
		if m.prefetchCB != nil {
			cb := m.prefetchCB
			go cb(lease.KID)
		}
	}

	// 90% 标记 critical 水位
	if usage >= criticalThreshold {
		lease.critical = true
	}
}

// Renew 为指定 KID 同步续租 lease。
// 调用 renew 函数获取新区间并更新 lease。
func (m *LeaseManager) Renew(kid string) error {
	if m.renew == nil {
		return errors.NonceExhausted().WithCause(fmt.Errorf("nonce: renew function not set"))
	}
	start, end, expiresAt, err := m.renew(kid)
	if err != nil {
		return errors.NonceExhausted().WithCause(err)
	}
	if end < start {
		return errors.NonceExhausted().WithCause(fmt.Errorf("nonce: invalid lease range [%d, %d]", start, end))
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.leases[kid] = &Lease{
		KID:       kid,
		Start:     start,
		End:       end,
		Next:      start,
		ExpiresAt: expiresAt,
	}
	return nil
}

// IsCritical 返回指定 KID 是否处于 critical 水位（使用量 >= 90%）。
func (m *LeaseManager) IsCritical(kid string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, ok := m.leases[kid]
	if !ok {
		return false
	}
	return lease.critical
}

// NonceToBytes 将 nonce 编号编码为 12 字节 big-endian nonce。
// uint64 占用低 8 字节，高 4 字节补零，符合 GCM 12 字节 nonce 标准。
func NonceToBytes(n uint64) [NonceSize]byte {
	var b [NonceSize]byte
	binary.BigEndian.PutUint64(b[4:], n)
	return b
}

// SingleflightLeaseManager 包装 LeaseManager，对相同 KID 的续租请求做 singleflight 合并（HA-12）。
//
// 在高并发场景下，多个 goroutine 可能同时发现 lease 耗尽并触发续租。
// singleflight 确保对同一 KID 的并发 Renew 调用只执行一次实际续租，
// 其余调用等待结果复用，避免向后端重复申请 lease。
type SingleflightLeaseManager struct {
	inner *LeaseManager
	group singleflight.Group
}

// NewSingleflightLeaseManager 创建 SingleflightLeaseManager。
func NewSingleflightLeaseManager(inner *LeaseManager) *SingleflightLeaseManager {
	return &SingleflightLeaseManager{inner: inner}
}

// Allocate 委托给内部 LeaseManager。
func (m *SingleflightLeaseManager) Allocate(kid string) (uint64, error) {
	return m.inner.Allocate(kid)
}

// Renew 使用 singleflight 合并相同 KID 的续租请求。
// 并发调用同一 KID 的 Renew 时，仅执行一次实际续租，其余等待复用结果。
func (m *SingleflightLeaseManager) Renew(kid string) error {
	_, err, _ := m.group.Do(kid, func() (interface{}, error) {
		return nil, m.inner.Renew(kid)
	})
	return err
}

// IsCritical 委托给内部 LeaseManager。
func (m *SingleflightLeaseManager) IsCritical(kid string) bool {
	return m.inner.IsCritical(kid)
}
