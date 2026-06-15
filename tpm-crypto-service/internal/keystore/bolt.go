package keystore

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
)

// BoltKeystore 是基于 BoltDB 的本地持久化实现。
type BoltKeystore struct {
	db *bolt.DB
	mu sync.Mutex // 仅保护 put/delete 的序列号生成
}

var (
	bucketMeta = []byte("meta") // key = keyID\0version -> DataKeyMeta (gob)
	bucketBlob = []byte("blob") // key = keyID\0version -> WrappedKey bytes
)

func NewBolt(path string) (*BoltKeystore, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt: %w", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, e1 := tx.CreateBucketIfNotExists(bucketMeta)
		_, e2 := tx.CreateBucketIfNotExists(bucketBlob)
		return errors.Join(e1, e2)
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &BoltKeystore{db: db}, nil
}

func (b *BoltKeystore) Close() error {
	if b.db == nil {
		return nil
	}
	return b.db.Close()
}

func (b *BoltKeystore) Put(_ context.Context, dek *WrappedDEK) error {
	if err := dek.Validate(); err != nil {
		return err
	}
	k := blobKey(dek.KeyID, dek.Version)
	metaBytes, err := gobEncode(meta{
		KeyID:         dek.KeyID,
		Algorithm:     dek.Algorithm,
		KeyLengthBits: dek.KeyLengthBits,
		Version:       dek.Version,
		Status:        dek.Status,
		CreatedAt:     dek.CreatedAt,
	})
	if err != nil {
		return err
	}
	return b.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketBlob).Put(k, dek.WrappedKey); err != nil {
			return err
		}
		return tx.Bucket(bucketMeta).Put(k, metaBytes)
	})
}

func (b *BoltKeystore) Get(_ context.Context, keyID string, version uint64) (*WrappedDEK, error) {
	var out *WrappedDEK
	err := b.db.View(func(tx *bolt.Tx) error {
		k := blobKey(keyID, version)
		raw := tx.Bucket(bucketBlob).Get(k)
		if raw == nil {
			return ErrNotFound
		}
		mb := tx.Bucket(bucketMeta).Get(k)
		if mb == nil {
			return ErrNotFound
		}
		m, err := gobDecode[meta](mb)
		if err != nil {
			return err
		}
		out = &WrappedDEK{
			KeyID:         m.KeyID,
			Algorithm:     m.Algorithm,
			KeyLengthBits: m.KeyLengthBits,
			Version:       m.Version,
			Status:        m.Status,
			CreatedAt:     m.CreatedAt,
			WrappedKey:    append([]byte(nil), raw...),
		}
		return nil
	})
	return out, err
}

func (b *BoltKeystore) GetLatestActive(_ context.Context, keyID string) (*WrappedDEK, error) {
	var out *WrappedDEK
	err := b.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketMeta).Cursor()
		prefix := []byte(keyID + "\x00")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			m, err := gobDecode[meta](v)
			if err != nil {
				return err
			}
			if m.Status != common.Active {
				continue
			}
			if out == nil || m.Version > out.Version {
				raw := tx.Bucket(bucketBlob).Get(k)
				if raw == nil {
					return ErrNotFound
				}
				out = &WrappedDEK{
					KeyID:         m.KeyID,
					Algorithm:     m.Algorithm,
					KeyLengthBits: m.KeyLengthBits,
					Version:       m.Version,
					Status:        m.Status,
					CreatedAt:     m.CreatedAt,
					WrappedKey:    append([]byte(nil), raw...),
				}
			}
		}
		if out == nil {
			return ErrNotFound
		}
		return nil
	})
	return out, err
}

func (b *BoltKeystore) List(_ context.Context) ([]DataKeyMeta, error) {
	var out []DataKeyMeta
	err := b.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).ForEach(func(_, v []byte) error {
			m, err := gobDecode[meta](v)
			if err != nil {
				return err
			}
			out = append(out, DataKeyMeta{
				KeyID:         m.KeyID,
				Algorithm:     m.Algorithm,
				KeyLengthBits: m.KeyLengthBits,
				Version:       m.Version,
				Status:        m.Status,
				CreatedAt:     m.CreatedAt,
			})
			return nil
		})
	})
	return out, err
}

func (b *BoltKeystore) Delete(_ context.Context, keyID string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucketMeta).Cursor()
		prefix := []byte(keyID + "\x00")
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			if err := tx.Bucket(bucketMeta).Delete(k); err != nil {
				return err
			}
			if err := tx.Bucket(bucketBlob).Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

type meta struct {
	KeyID         string
	Algorithm     common.Algorithm
	KeyLengthBits uint32
	Version       uint64
	Status        common.KeyStatus
	CreatedAt     time.Time
}

func blobKey(keyID string, version uint64) []byte {
	return []byte(fmt.Sprintf("%s\x00%020d", keyID, version))
}

func gobEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gobDecode[T any](b []byte) (T, error) {
	var v T
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&v); err != nil {
		return v, err
	}
	return v, nil
}
