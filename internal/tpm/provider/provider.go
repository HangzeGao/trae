// Package provider defines the TPMProvider interface and a software-backed
// implementation for P0 testing. Per design §6.3 and §20.4, P0 may use
// swtpm; in environments without swtpm, a software provider that emulates
// seal/unseal with AES-256-GCM is acceptable for P0 testing.
//
// The software provider persists NRWK and CRK envelopes to disk so that
// restart recovery works (design §19.1: "swtpm 创建 NRWK、封装/解封 CRK、
// 重启后恢复"). It is NOT a security boundary; production deployments MUST
// use a real TPM or swtpm with proper isolation.
package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kvlt/key-vault/internal/crypto/aad"
)

// CRKEnvelope is the sealed CRK material for a node.
type CRKEnvelope struct {
	NodeID         string    `json:"node_id"`
	ClusterID      string    `json:"cluster_id"`
	PlaneRole      string    `json:"plane_role"`
	CRKVersion     uint32    `json:"crk_version"`
	NRWKName       string    `json:"nrwk_name"`
	WrappedCRK     []byte    `json:"wrapped_crk"`     // AES-GCM sealed
	Nonce          []byte    `json:"nonce"`
	Tag            []byte    `json:"tag"`
	BaselineDigest []byte    `json:"baseline_digest"`
	PolicyDigest   []byte    `json:"policy_digest"`
}

// TPMObjectRef references an NRWK in the provider.
type TPMObjectRef struct {
	Name      string `json:"name"`
	KeyBytes  []byte `json:"key_bytes"` // software provider only; real TPM never exposes
}

// Provider is the TPM abstraction.
type Provider interface {
	// EnsureNRWK creates or loads the NRWK for the given name.
	EnsureNRWK(ctx context.Context, name string) (*TPMObjectRef, error)
	// SealCRK encrypts the CRK plaintext under the NRWK, bound to the AAD.
	SealCRK(ctx context.Context, nrwk *TPMObjectRef, crk []byte, aad aad.CRKAAD) (*CRKEnvelope, error)
	// UnsealCRK decrypts the CRK envelope, verifying the AAD.
	UnsealCRK(ctx context.Context, nrwk *TPMObjectRef, env *CRKEnvelope, aad aad.CRKAAD) ([]byte, error)
	// Quote is a P1 stub; P0 returns a fixed placeholder.
	Quote(ctx context.Context, nonce []byte, pcrs []int) ([]byte, error)
	// Close releases provider resources.
	Close() error
}

// SoftwareProvider is a disk-backed software TPM emulator for P0 testing.
type SoftwareProvider struct {
	mu       sync.Mutex
	stateDir string
	nrwk     map[string]*TPMObjectRef
}

// NewSoftwareProvider constructs a software provider backed by stateDir.
func NewSoftwareProvider(stateDir string) (*SoftwareProvider, error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("provider: mkdir: %w", err)
	}
	sp := &SoftwareProvider{
		stateDir: stateDir,
		nrwk:     make(map[string]*TPMObjectRef),
	}
	if err := sp.loadAll(); err != nil {
		return nil, err
	}
	return sp, nil
}

func (s *SoftwareProvider) loadAll() error {
	entries, err := os.ReadDir(s.stateDir)
	if err != nil {
		return fmt.Errorf("provider: readdir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".nrwk" {
			continue
		}
		path := filepath.Join(s.stateDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("provider: read %s: %w", path, err)
		}
		var ref TPMObjectRef
		if err := json.Unmarshal(b, &ref); err != nil {
			return fmt.Errorf("provider: parse %s: %w", path, err)
		}
		s.nrwk[ref.Name] = &ref
	}
	return nil
}

// EnsureNRWK creates or loads the NRWK for the given name.
func (s *SoftwareProvider) EnsureNRWK(ctx context.Context, name string) (*TPMObjectRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ref, ok := s.nrwk[name]; ok {
		return ref, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("provider: rand: %w", err)
	}
	ref := &TPMObjectRef{Name: name, KeyBytes: key}
	if err := s.persistNRWK(ref); err != nil {
		return nil, err
	}
	s.nrwk[name] = ref
	return ref, nil
}

func (s *SoftwareProvider) persistNRWK(ref *TPMObjectRef) error {
	path := filepath.Join(s.stateDir, ref.Name+".nrwk")
	b, err := json.Marshal(ref)
	if err != nil {
		return fmt.Errorf("provider: marshal: %w", err)
	}
	// Write atomically.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("provider: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// SealCRK encrypts the CRK under the NRWK with AES-256-GCM, binding the AAD.
func (s *SoftwareProvider) SealCRK(ctx context.Context, nrwk *TPMObjectRef, crk []byte, a aad.CRKAAD) (*CRKEnvelope, error) {
	if len(nrwk.KeyBytes) != 32 {
		return nil, fmt.Errorf("provider: NRWK key length %d", len(nrwk.KeyBytes))
	}
	block, err := aes.NewCipher(nrwk.KeyBytes)
	if err != nil {
		return nil, fmt.Errorf("provider: aes new: %w", err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("provider: gcm new: %w", err)
	}
	aadBytes, err := a.Canonical()
	if err != nil {
		return nil, fmt.Errorf("provider: aad canonical: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("provider: rand nonce: %w", err)
	}
	sealed := g.Seal(nil, nonce, crk, aadBytes)
	tagLen := g.Overhead()
	ct := sealed[:len(sealed)-tagLen]
	tag := sealed[len(sealed)-tagLen:]
	return &CRKEnvelope{
		NodeID:         a.NodeID,
		ClusterID:      a.ClusterID,
		PlaneRole:      a.PlaneRole,
		CRKVersion:     a.CRKVersion,
		NRWKName:       nrwk.Name,
		WrappedCRK:     ct,
		Nonce:          nonce,
		Tag:            tag,
		BaselineDigest: a.BaselineDigest,
		PolicyDigest:   a.PolicyDigest,
	}, nil
}

// UnsealCRK decrypts the CRK envelope, verifying the AAD.
func (s *SoftwareProvider) UnsealCRK(ctx context.Context, nrwk *TPMObjectRef, env *CRKEnvelope, a aad.CRKAAD) ([]byte, error) {
	if env.NRWKName != nrwk.Name {
		return nil, fmt.Errorf("provider: NRWK name mismatch")
	}
	if len(nrwk.KeyBytes) != 32 {
		return nil, fmt.Errorf("provider: NRWK key length %d", len(nrwk.KeyBytes))
	}
	block, err := aes.NewCipher(nrwk.KeyBytes)
	if err != nil {
		return nil, fmt.Errorf("provider: aes new: %w", err)
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("provider: gcm new: %w", err)
	}
	aadBytes, err := a.Canonical()
	if err != nil {
		return nil, fmt.Errorf("provider: aad canonical: %w", err)
	}
	combined := make([]byte, 0, len(env.WrappedCRK)+len(env.Tag))
	combined = append(combined, env.WrappedCRK...)
	combined = append(combined, env.Tag...)
	pt, err := g.Open(nil, env.Nonce, combined, aadBytes)
	if err != nil {
		return nil, fmt.Errorf("provider: unseal failed (AAD or integrity check)")
	}
	return pt, nil
}

// Quote is a P1 stub.
func (s *SoftwareProvider) Quote(ctx context.Context, nonce []byte, pcrs []int) ([]byte, error) {
	h := sha256.New()
	h.Write([]byte("swtpm-quote"))
	h.Write(nonce)
	for _, p := range pcrs {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(p))
		h.Write(b[:])
	}
	return h.Sum(nil), nil
}

// Close is a no-op for the software provider.
func (s *SoftwareProvider) Close() error { return nil }

// ErrProviderUnavailable is returned when the provider cannot service a request.
var ErrProviderUnavailable = errors.New("provider: unavailable")
