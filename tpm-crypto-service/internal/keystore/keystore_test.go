package keystore

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

func TestMemoryRoundTrip(t *testing.T) {
	ks := NewMemory()
	defer ks.Close()
	dek := &WrappedDEK{
		KeyID:         "k1",
		Algorithm:     common.AES,
		KeyLengthBits: 256,
		Version:       1,
		Status:        common.Active,
		CreatedAt:     time.Now(),
		WrappedKey:    bytes.Repeat([]byte{0xAA}, 64),
	}
	require.NoError(t, ks.Put(context.Background(), dek))

	got, err := ks.Get(context.Background(), "k1", 1)
	require.NoError(t, err)
	require.Equal(t, dek.KeyID, got.KeyID)
	require.Equal(t, dek.WrappedKey, got.WrappedKey)
}

func TestMemoryGetLatestActive(t *testing.T) {
	ks := NewMemory()
	defer ks.Close()
	// v1 active, v2 active, v3 retired
	for v := uint64(1); v <= 3; v++ {
		st := common.Active
		if v == 3 {
			st = common.Retired
		}
		require.NoError(t, ks.Put(context.Background(), &WrappedDEK{
			KeyID:         "k",
			Algorithm:     common.AES,
			KeyLengthBits: 128,
			Version:       v,
			Status:        st,
			CreatedAt:     time.Now(),
			WrappedKey:    []byte{0x01, 0x02},
		}))
	}
	got, err := ks.GetLatestActive(context.Background(), "k")
	require.NoError(t, err)
	require.Equal(t, uint64(2), got.Version)
}

func TestMemoryConflictOnDuplicateVersion(t *testing.T) {
	ks := NewMemory()
	defer ks.Close()
	d := &WrappedDEK{KeyID: "k", Algorithm: common.AES, KeyLengthBits: 128, Version: 1, Status: common.Active, CreatedAt: time.Now(), WrappedKey: []byte{0x01}}
	require.NoError(t, ks.Put(context.Background(), d))
	err := ks.Put(context.Background(), d)
	require.ErrorIs(t, err, ErrConflict)
}

func TestBoltRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewBolt(dir + "/data.bolt")
	require.NoError(t, err)
	defer ks.Close()
	dek := &WrappedDEK{
		KeyID:         "kk",
		Algorithm:     common.SM4,
		KeyLengthBits: 128,
		Version:       1,
		Status:        common.Active,
		CreatedAt:     time.Now(),
		WrappedKey:    bytes.Repeat([]byte{0xCC}, 48),
	}
	require.NoError(t, ks.Put(context.Background(), dek))

	// 重启模拟
	ks.Close()
	ks2, err := NewBolt(dir + "/data.bolt")
	require.NoError(t, err)
	defer ks2.Close()
	got, err := ks2.Get(context.Background(), "kk", 1)
	require.NoError(t, err)
	require.Equal(t, dek.WrappedKey, got.WrappedKey)
}

func TestBoltGetLatestActive(t *testing.T) {
	dir := t.TempDir()
	ks, err := NewBolt(dir + "/data.bolt")
	require.NoError(t, err)
	defer ks.Close()
	for v := uint64(1); v <= 4; v++ {
		st := common.Active
		if v == 4 {
			st = common.Retired
		}
		require.NoError(t, ks.Put(context.Background(), &WrappedDEK{
			KeyID:         "x",
			Algorithm:     common.AES,
			KeyLengthBits: 256,
			Version:       v,
			Status:        st,
			CreatedAt:     time.Now(),
			WrappedKey:    []byte{0x00, 0x01},
		}))
	}
	got, err := ks.GetLatestActive(context.Background(), "x")
	require.NoError(t, err)
	require.Equal(t, uint64(3), got.Version)
}

func TestBoltDelete(t *testing.T) {
	dir := t.TempDir()
	ks, _ := NewBolt(dir + "/data.bolt")
	defer ks.Close()
	_ = ks.Put(context.Background(), &WrappedDEK{
		KeyID: "y", Algorithm: common.AES, KeyLengthBits: 128, Version: 1, Status: common.Active, CreatedAt: time.Now(), WrappedKey: []byte{0x01},
	})
	_ = ks.Put(context.Background(), &WrappedDEK{
		KeyID: "y", Algorithm: common.AES, KeyLengthBits: 128, Version: 2, Status: common.Active, CreatedAt: time.Now(), WrappedKey: []byte{0x02},
	})
	require.NoError(t, ks.Delete(context.Background(), "y"))
	_, err := ks.Get(context.Background(), "y", 1)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = ks.Get(context.Background(), "y", 2)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestEtcdStub(t *testing.T) {
	ks, err := NewEtcd(nil, "/test/prefix/")
	require.NoError(t, err)
	require.NoError(t, ks.Put(context.Background(), &WrappedDEK{
		KeyID: "e", Algorithm: common.SM4, KeyLengthBits: 128, Version: 1, Status: common.Active, CreatedAt: time.Now(), WrappedKey: []byte{0xAB},
	}))
	got, err := ks.Get(context.Background(), "e", 1)
	require.NoError(t, err)
	require.Equal(t, []byte{0xAB}, got.WrappedKey)
	_ = errors.New // 保持 errors 引用
	_ = sync.Mutex{} // 保持 sync 引用
}
