// Package keystore 提供数据密钥(DEK)的密文存储抽象。
//
// 同一接口下支持三种后端:
//   - memory:进程内存(默认/测试)
//   - bolt:本地嵌入式 KV(BoltDB)
//   - etcd:集群共享 KV(生产多副本)
package keystore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

// DataKeyMeta 是数据密钥的元数据(对外可见,不含密钥材料)。
type DataKeyMeta struct {
	KeyID         string
	Algorithm     common.Algorithm
	KeyLengthBits uint32
	Version       uint64
	Status        common.KeyStatus
	CreatedAt     time.Time
	// 仅在 List/Get 时填充,WrappedKey 通过单独方法获取。
}

// WrappedDEK 是 DEK 的密文形态。
type WrappedDEK struct {
	KeyID         string
	Algorithm     common.Algorithm
	KeyLengthBits uint32
	Version       uint64
	Status        common.KeyStatus
	CreatedAt     time.Time
	// WrappedKey 是密文+nonce+tag 的拼接(GCM 输出)
	WrappedKey []byte
}

// Errors
var (
	ErrNotFound  = errors.New("keystore: not found")
	ErrConflict  = errors.New("keystore: version conflict")
	ErrEmptyKey  = errors.New("keystore: empty key id")
)

// Keystore 是 DEK 存储的统一接口。
type Keystore interface {
	Put(ctx context.Context, dek *WrappedDEK) error
	Get(ctx context.Context, keyID string, version uint64) (*WrappedDEK, error)
	GetLatestActive(ctx context.Context, keyID string) (*WrappedDEK, error)
	List(ctx context.Context) ([]DataKeyMeta, error)
	Delete(ctx context.Context, keyID string) error
	Close() error
}

// Validate 对 DEK 进行基本校验。
func (d *WrappedDEK) Validate() error {
	if d.KeyID == "" {
		return ErrEmptyKey
	}
	if d.WrappedKey == nil {
		return fmt.Errorf("wrapped key required")
	}
	if d.KeyLengthBits != 128 && d.KeyLengthBits != 192 && d.KeyLengthBits != 256 {
		return fmt.Errorf("invalid key length %d", d.KeyLengthBits)
	}
	return nil
}

// MemoryKeystore 是进程内的 Map,主要用于单实例与单元测试。
type MemoryKeystore struct {
	mu    sync.RWMutex
	store map[string]map[uint64]*WrappedDEK // keyID -> version -> DEK
}

func NewMemory() *MemoryKeystore {
	return &MemoryKeystore{store: make(map[string]map[uint64]*WrappedDEK)}
}

func (m *MemoryKeystore) Put(_ context.Context, dek *WrappedDEK) error {
	if err := dek.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	vs := m.store[dek.KeyID]
	if vs == nil {
		vs = make(map[uint64]*WrappedDEK)
		m.store[dek.KeyID] = vs
	}
	if _, exists := vs[dek.Version]; exists {
		return ErrConflict
	}
	cp := *dek
	cp.WrappedKey = append([]byte(nil), dek.WrappedKey...)
	vs[dek.Version] = &cp
	return nil
}

func (m *MemoryKeystore) Get(_ context.Context, keyID string, version uint64) (*WrappedDEK, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vs, ok := m.store[keyID]
	if !ok {
		return nil, ErrNotFound
	}
	d, ok := vs[version]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *d
	cp.WrappedKey = append([]byte(nil), d.WrappedKey...)
	return &cp, nil
}

func (m *MemoryKeystore) GetLatestActive(_ context.Context, keyID string) (*WrappedDEK, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	vs, ok := m.store[keyID]
	if !ok {
		return nil, ErrNotFound
	}
	var best *WrappedDEK
	for _, d := range vs {
		if d.Status != common.Active {
			continue
		}
		if best == nil || d.Version > best.Version {
			best = d
		}
	}
	if best == nil {
		return nil, ErrNotFound
	}
	cp := *best
	cp.WrappedKey = append([]byte(nil), best.WrappedKey...)
	return &cp, nil
}

func (m *MemoryKeystore) List(_ context.Context) ([]DataKeyMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]DataKeyMeta, 0)
	for _, vs := range m.store {
		for _, d := range vs {
			out = append(out, DataKeyMeta{
				KeyID:         d.KeyID,
				Algorithm:     d.Algorithm,
				KeyLengthBits: d.KeyLengthBits,
				Version:       d.Version,
				Status:        d.Status,
				CreatedAt:     d.CreatedAt,
			})
		}
	}
	return out, nil
}

func (m *MemoryKeystore) Delete(_ context.Context, keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, keyID)
	return nil
}

func (m *MemoryKeystore) Close() error { return nil }
