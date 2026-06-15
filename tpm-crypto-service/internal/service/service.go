// Package service 实现 CryptoService 的业务用例。
package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"

	"github.com/tpm-crypto/tpm-crypto-service/internal/audit"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto"
	"github.com/tpm-crypto/tpm-crypto-service/internal/crypto/common"
	"github.com/tpm-crypto/tpm-crypto-service/internal/keystore"
	"github.com/tpm-crypto/tpm-crypto-service/internal/obs"
	"github.com/tpm-crypto/tpm-crypto-service/internal/tpm"
)

// Service 是业务层入口,组合 Keystore + SRK + Crypto Factory + Metrics。
type Service struct {
	KS        keystore.Keystore
	SRK       *tpm.SRK
	Factory   crypto.Factory
	ARK       []byte // 运行时 ARK 明文
	Log       *zap.Logger
	Metrics   *obs.Metrics
	TPMMode   string
	Caller    string // 标识当前服务实例,用于审计
}

// Health 表示业务层健康状态。
type Health struct {
	TPMReady  bool
	ARKReady  bool
	KSReady   bool
	Mode      string
}

// HealthCheck 聚合三组件状态。
func (s *Service) HealthCheck(_ context.Context) Health {
	h := Health{
		TPMReady: s.SRK != nil,
		ARKReady: len(s.ARK) == 32,
		KSReady:  s.KS != nil,
		Mode:     s.TPMMode,
	}
	return h
}

// CreateDataKey 创建一条新的数据密钥。
func (s *Service) CreateDataKey(ctx context.Context, keyID string, alg common.Algorithm, keyLenBits uint32) (*keystore.DataKeyMeta, error) {
	op := "create_data_key"
	t0 := time.Now()
	defer func() { s.observe(op, alg.String(), "", time.Since(t0)) }()

	if keyID == "" {
		return nil, fmt.Errorf("%w: key_id required", common.ErrInvalidArgument)
	}
	if alg != common.AES && alg != common.SM4 {
		return nil, fmt.Errorf("%w: unsupported algorithm %s", common.ErrUnsupported, alg)
	}
	if alg == common.SM4 && keyLenBits != 128 {
		return nil, fmt.Errorf("%w: SM4 must be 128-bit", common.ErrInvalidArgument)
	}
	if alg == common.AES && keyLenBits != 128 && keyLenBits != 192 && keyLenBits != 256 {
		return nil, fmt.Errorf("%w: AES must be 128/192/256-bit", common.ErrInvalidArgument)
	}
	// 确认 keyID 未被占用(若存在则视为 INVALID_ARGUMENT,业务侧需先 rotate/revoke)
	if _, err := s.KS.GetLatestActive(ctx, keyID); err == nil {
		return nil, fmt.Errorf("%w: key_id %s", common.ErrKeyIDExists, keyID)
	}

	rawKey := make([]byte, keyLenBits/8)
	if _, err := io.ReadFull(rand.Reader, rawKey); err != nil {
		return nil, err
	}
	wrapped, err := s.wrapDEKWithID(keyID, alg, keyLenBits, rawKey)
	if err != nil {
		return nil, err
	}
	wrapped.Version = 1
	wrapped.Status = common.Active
	wrapped.CreatedAt = time.Now()
	if err := s.KS.Put(ctx, wrapped); err != nil {
		return nil, err
	}
	meta := &keystore.DataKeyMeta{
		KeyID:         wrapped.KeyID,
		Algorithm:     wrapped.Algorithm,
		KeyLengthBits: wrapped.KeyLengthBits,
		Version:       wrapped.Version,
		Status:        wrapped.Status,
		CreatedAt:     wrapped.CreatedAt,
	}
	audit.Log(s.Log, op, keyID, alg.String(), "", s.Caller, "ok")
	return meta, nil
}

// DescribeDataKey 返回元数据(不含密钥材料)。
func (s *Service) DescribeDataKey(ctx context.Context, keyID string, version uint64) (*keystore.DataKeyMeta, error) {
	op := "describe_data_key"
	t0 := time.Now()
	defer func() { s.observe(op, "", "", time.Since(t0)) }()

	if keyID == "" {
		return nil, fmt.Errorf("%w: key_id required", common.ErrInvalidArgument)
	}
	var (
		w   *keystore.WrappedDEK
		err error
	)
	if version == 0 {
		w, err = s.KS.GetLatestActive(ctx, keyID)
	} else {
		w, err = s.KS.Get(ctx, keyID, version)
	}
	if err != nil {
		return nil, err
	}
	meta := &keystore.DataKeyMeta{
		KeyID:         w.KeyID,
		Algorithm:     w.Algorithm,
		KeyLengthBits: w.KeyLengthBits,
		Version:       w.Version,
		Status:        w.Status,
		CreatedAt:     w.CreatedAt,
	}
	audit.Log(s.Log, op, keyID, w.Algorithm.String(), "", s.Caller, "ok")
	return meta, nil
}

// RotateDataKey 创建一个新版本的 DEK,旧版本标记为 retired。
func (s *Service) RotateDataKey(ctx context.Context, keyID string) (*keystore.DataKeyMeta, error) {
	op := "rotate_data_key"
	t0 := time.Now()
	defer func() { s.observe(op, "", "", time.Since(t0)) }()

	if keyID == "" {
		return nil, fmt.Errorf("%w: key_id required", common.ErrInvalidArgument)
	}
	latest, err := s.KS.GetLatestActive(ctx, keyID)
	if err != nil {
		return nil, err
	}
	// 旧版本置为 retired(直接 Put 覆盖,keystore 接受同 keyID 不同 version)
	retired := *latest
	retired.Status = common.Retired
	if err := s.KS.Put(ctx, &retired); err != nil {
		return nil, err
	}
	// 创建新版本
	rawKey := make([]byte, latest.KeyLengthBits/8)
	if _, err := io.ReadFull(rand.Reader, rawKey); err != nil {
		return nil, err
	}
	wrapped, err := s.wrapDEKWithID(keyID, latest.Algorithm, latest.KeyLengthBits, rawKey)
	if err != nil {
		return nil, err
	}
	wrapped.Version = latest.Version + 1
	wrapped.Status = common.Active
	wrapped.CreatedAt = time.Now()
	if err := s.KS.Put(ctx, wrapped); err != nil {
		return nil, err
	}
	meta := &keystore.DataKeyMeta{
		KeyID:         wrapped.KeyID,
		Algorithm:     wrapped.Algorithm,
		KeyLengthBits: wrapped.KeyLengthBits,
		Version:       wrapped.Version,
		Status:        wrapped.Status,
		CreatedAt:     wrapped.CreatedAt,
	}
	audit.Log(s.Log, op, keyID, wrapped.Algorithm.String(), "", s.Caller, "ok")
	return meta, nil
}

// RevokeDataKey 把指定 keyID 的所有 version(或指定 version)标记为 revoked。
func (s *Service) RevokeDataKey(ctx context.Context, keyID string, version uint64) (uint32, error) {
	op := "revoke_data_key"
	t0 := time.Now()
	defer func() { s.observe(op, "", "", time.Since(t0)) }()

	if keyID == "" {
		return 0, fmt.Errorf("%w: key_id required", common.ErrInvalidArgument)
	}
	// 简化实现:支持 version=0 时删除整个 keyID(物理删除)
	// 也支持具体 version 时标记 revoked(此处为简化,直接 Delete 整个 keyID)
	if version == 0 {
		// 列出后逐个 revoke 标记
		metas, err := s.KS.List(ctx)
		if err != nil {
			return 0, err
		}
		var cnt uint32
		for _, m := range metas {
			if m.KeyID != keyID {
				continue
			}
			w, err := s.KS.Get(ctx, m.KeyID, m.Version)
			if err != nil {
				continue
			}
			w.Status = common.Revoked
			if err := s.KS.Put(ctx, w); err != nil {
				return cnt, err
			}
			cnt++
		}
		audit.Log(s.Log, op, keyID, "", "", s.Caller, "ok")
		return cnt, nil
	}
	w, err := s.KS.Get(ctx, keyID, version)
	if err != nil {
		return 0, err
	}
	w.Status = common.Revoked
	if err := s.KS.Put(ctx, w); err != nil {
		return 0, err
	}
	audit.Log(s.Log, op, keyID, w.Algorithm.String(), "", s.Caller, "ok")
	return 1, nil
}

// Encrypt 使用指定 (keyID, version, alg, mode) 对 plaintext 进行加密。
func (s *Service) Encrypt(ctx context.Context, keyID string, version uint64, alg common.Algorithm, mode common.BlockMode, plain, aad []byte) (cipher, iv, tag []byte, usedVersion uint64, err error) {
	op := "encrypt"
	t0 := time.Now()
	defer func() { s.observe(op, alg.String(), mode.String(), time.Since(t0)) }()

	raw, usedV, err := s.loadDEK(ctx, keyID, version, alg)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	c, _, err := s.Factory(alg, mode, len(raw))
	if err != nil {
		return nil, nil, nil, 0, err
	}
	ct, ivOut, tagOut, err := c.Encrypt(raw, plain, nil, aad)
	if err != nil {
		audit.Log(s.Log, op, keyID, alg.String(), mode.String(), s.Caller, "err", zap.Error(err))
		return nil, nil, nil, 0, err
	}
	audit.Log(s.Log, op, keyID, alg.String(), mode.String(), s.Caller, "ok")
	return ct, ivOut, tagOut, usedV, nil
}

// Decrypt 是 Encrypt 的逆过程。
func (s *Service) Decrypt(ctx context.Context, keyID string, version uint64, alg common.Algorithm, mode common.BlockMode, ciphertext, iv, tag, aad []byte) ([]byte, error) {
	op := "decrypt"
	t0 := time.Now()
	defer func() { s.observe(op, alg.String(), mode.String(), time.Since(t0)) }()

	raw, _, err := s.loadDEK(ctx, keyID, version, alg)
	if err != nil {
		return nil, err
	}
	c, _, err := s.Factory(alg, mode, len(raw))
	if err != nil {
		return nil, err
	}
	pt, err := c.Decrypt(raw, ciphertext, iv, tag, aad)
	if err != nil {
		audit.Log(s.Log, op, keyID, alg.String(), mode.String(), s.Caller, "err", zap.Error(err))
		return nil, err
	}
	audit.Log(s.Log, op, keyID, alg.String(), mode.String(), s.Caller, "ok")
	return pt, nil
}

// loadDEK 从 keystore 取出 wrapped DEK 并通过 ARK 解封为明文。
func (s *Service) loadDEK(ctx context.Context, keyID string, version uint64, alg common.Algorithm) ([]byte, uint64, error) {
	if len(s.ARK) != 32 {
		return nil, 0, fmt.Errorf("%w: ARK not unsealed", common.ErrAuth)
	}
	var (
		w   *keystore.WrappedDEK
		err error
	)
	if version == 0 {
		w, err = s.KS.GetLatestActive(ctx, keyID)
	} else {
		w, err = s.KS.Get(ctx, keyID, version)
	}
	if err != nil {
		return nil, 0, err
	}
	if w.Algorithm != alg {
		return nil, 0, fmt.Errorf("%w: algorithm mismatch: stored=%s requested=%s",
			common.ErrInvalidArgument, w.Algorithm, alg)
	}
	if w.Status == common.Revoked {
		return nil, 0, fmt.Errorf("%w: key_id %s version %d",
			common.ErrKeyRevoked, keyID, w.Version)
	}
	pt, err := s.unwrapDEK(keyID, w)
	if err != nil {
		return nil, 0, err
	}
	return pt, w.Version, nil
}

// wrapDEKWithID 使用 ARK 作为 KEK,采用 GCM 模式;非密钥业务(AAD=keyID)防篡改。
func (s *Service) wrapDEKWithID(keyID string, alg common.Algorithm, keyLenBits uint32, rawKey []byte) (*keystore.WrappedDEK, error) {
	c, _, err := s.Factory(alg, common.GCM, 32) // ARK = 32B
	if err != nil {
		return nil, err
	}
	wrapped, iv, tag, err := c.Encrypt(s.ARK, rawKey, nil, []byte(keyID))
	if err != nil {
		return nil, err
	}
	// 把 iv + tag 拼到 wrapped 里方便落盘
	out := append([]byte{}, wrapped...)
	out = append(out, iv...)
	out = append(out, tag...)
	return &keystore.WrappedDEK{
		KeyID:         keyID,
		Algorithm:     alg,
		KeyLengthBits: keyLenBits,
		WrappedKey:    out,
	}, nil
}

func (s *Service) unwrapDEK(keyID string, w *keystore.WrappedDEK) ([]byte, error) {
	if len(w.WrappedKey) < 28 { // 至少 12B iv + 16B tag
		return nil, fmt.Errorf("%w: wrapped key too short", common.ErrInvalidArgument)
	}
	c, _, err := s.Factory(w.Algorithm, common.GCM, 32)
	if err != nil {
		return nil, err
	}
	iv := w.WrappedKey[len(w.WrappedKey)-28 : len(w.WrappedKey)-16]
	tag := w.WrappedKey[len(w.WrappedKey)-16:]
	ct := w.WrappedKey[:len(w.WrappedKey)-28]
	pt, err := c.Decrypt(s.ARK, ct, iv, tag, []byte(keyID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", common.ErrAuth, err)
	}
	return pt, nil
}

func (s *Service) observe(op, alg, mode string, d time.Duration) {
	if s.Metrics != nil {
		s.Metrics.OpDuration.WithLabelValues(op, alg, mode, "ok").Observe(d.Seconds())
	}
}
