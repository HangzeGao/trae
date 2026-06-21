// Package crypto implements the Crypto application service per design §9.4, §9.5, §9.7.
package crypto

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kvlt/key-vault/internal/crypto/aad"
	"github.com/kvlt/key-vault/internal/crypto/aead"
	"github.com/kvlt/key-vault/internal/crypto/envelope"
	"github.com/kvlt/key-vault/internal/crypto/nonce"
	keystate "github.com/kvlt/key-vault/internal/domain/key/state"
	"github.com/kvlt/key-vault/internal/domain/policy"
	"github.com/kvlt/key-vault/internal/errorsx"
	"github.com/kvlt/key-vault/internal/repository/models"
	"github.com/kvlt/key-vault/internal/resolver/keyresolver"
)

// EncryptCommand is the input for Encrypt.
type EncryptCommand struct {
	TenantID   string
	KeyID      string
	Plaintext  []byte
	AAD        CallerAADInput
	NodeID     string
	PrincipalID string
}

// CallerAADInput is the caller-provided AAD.
type CallerAADInput struct {
	ResourceID string
	Purpose    string
}

// EncryptResult is the output of Encrypt.
type EncryptResult struct {
	KeyID      string
	KeyVersion uint32
	SuiteID    string
	Ciphertext []byte // Envelope v1 bytes
}

// DecryptCommand is the input for Decrypt.
type DecryptCommand struct {
	TenantID   string
	Ciphertext []byte // Envelope v1 bytes
	AAD        CallerAADInput
	PrincipalID string
}

// DecryptResult is the output of Decrypt.
type DecryptResult struct {
	KeyID      string
	KeyVersion uint32
	Plaintext  []byte
}

// GenerateDataKeyCommand is the input for GenerateDataKey.
type GenerateDataKeyCommand struct {
	TenantID   string
	KeyID      string
	Purpose    string
	TTL        time.Duration
	EncryptionContext map[string]string
	PrincipalID string
	Caller     string // "sdk" | "direct"
}

// DataKeyResult is the output of GenerateDataKey.
type DataKeyResult struct {
	KeyID                 string
	KeyVersion            uint32
	SuiteID               string
	PlaintextDataKey      []byte
	WrappedDataKey        []byte
	ClientZeroizeBy       time.Time
	EncryptionContextHash string
}

// Store is the repository subset used by the crypto service.
type Store interface {
	GetKey(ctx context.Context, tenantID, keyID string) (*models.Key, error)
	GetKeyVersionByNo(ctx context.Context, keyID string, versionNo uint32) (*models.KeyVersion, error)
	GetCurrentKeyVersion(ctx context.Context, keyID string) (*models.KeyVersion, error)
	AllocateNonceRange(ctx context.Context, keyVersionID, nodeID string, domain uint32, size uint64, ttl time.Duration) (*models.NonceLease, error)
	UpdateNonceUsed(ctx context.Context, leaseID string, used uint64) error
	GetNonceLease(ctx context.Context, leaseID string) (*models.NonceLease, error)
	ClusterEpoch(ctx context.Context) (uint64, error)
}

// Service is the crypto application service.
type Service struct {
	mu          sync.Mutex
	store       Store
	resolver    *keyresolver.Resolver
	policies    *policy.Engine
	nonceMgr    *nonce.Manager
	maxBodySize int
	dataKeyMaxTTL time.Duration
	dataKeyQuota  int
	tenantCallCounts map[string]int // P0 simple in-memory quota counter
	quotaWindowStart map[string]time.Time
}

// New constructs a crypto service.
func New(store Store, resolver *keyresolver.Resolver, policies *policy.Engine,
	nonceMgr *nonce.Manager, maxBodySize int, dataKeyMaxTTL time.Duration, dataKeyQuota int) *Service {
	return &Service{
		store:            store,
		resolver:         resolver,
		policies:         policies,
		nonceMgr:         nonceMgr,
		maxBodySize:      maxBodySize,
		dataKeyMaxTTL:    dataKeyMaxTTL,
		dataKeyQuota:     dataKeyQuota,
		tenantCallCounts: make(map[string]int),
		quotaWindowStart: make(map[string]time.Time),
	}
}

// Encrypt performs server-side encryption per design §9.4.
func (s *Service) Encrypt(ctx context.Context, cmd EncryptCommand) (*EncryptResult, error) {
	if len(cmd.Plaintext) > s.maxBodySize {
		return nil, errorsx.New(errorsx.CodeBadRequest, "plaintext exceeds max size", false)
	}
	k, err := s.store.GetKey(ctx, cmd.TenantID, cmd.KeyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	if !keystate.CanEncrypt(keystate.KeyStatus(k.Status)) {
		return nil, errorsx.New(errorsx.CodeKeyDisabled, "key not active for encryption", false)
	}
	// Validate suite against policy.
	pol, err := s.policies.Get(k.PolicyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "policy not found", false)
	}
	suite, err := pol.SuiteByID(k.SuiteID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "suite not in policy", false)
	}
	if !policy.CanEncrypt(suite.Status) {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "suite not available for encryption", false)
	}
	suiteEnum, _ := suiteIDFromString(k.SuiteID)

	// Get current key version.
	kv, err := s.store.GetCurrentKeyVersion(ctx, cmd.KeyID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "version fetch failed", false, err)
	}
	if !keystate.KVCanEncrypt(keystate.KeyVersionStatus(kv.Status)) {
		return nil, errorsx.New(errorsx.CodeKeyDisabled, "key version not active", false)
	}

	// Issue DEK lease (CRK critical section inside resolver).
	lease, err := s.resolver.IssueDEKLease(ctx, kv.ID, k.ID, kv.VersionNo, suiteEnum,
		cmd.TenantID, cmd.AAD.Purpose, cmd.NodeID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeTPMUnavailable, "dek lease failed", true, err)
	}

	// Get nonce lease.
	epoch, err := s.store.ClusterEpoch(ctx)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "epoch fetch failed", false, err)
	}
	nonceLease, err := s.nonceMgr.GetLeaseForUse(cmd.NodeID, kv.ID, epoch)
	if err != nil {
		if errors.Is(err, nonce.ErrFrozen) {
			return nil, errorsx.New(errorsx.CodeNodeFrozen, "node frozen", false)
		}
		return nil, errorsx.Wrap(errorsx.CodeNonceExhausted, "nonce lease failed", true, err)
	}
	nonceBytes, err := s.nonceMgr.NextNonce(nonceLease.LeaseID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeNonceExhausted, "nonce exhausted", true, err)
	}

	// Build caller AAD.
	callerAAD, err := aad.CallerAAD{
		TenantID:   cmd.TenantID,
		KeyID:      cmd.KeyID,
		KeyVersion: kv.VersionNo,
		Purpose:    cmd.AAD.Purpose,
		SuiteID:    uint16(suiteEnum),
		ResourceID: cmd.AAD.ResourceID,
	}.Canonical()
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "aad canonical failed", false, err)
	}

	// Seal envelope.
	envBytes, err := envelope.Seal(suiteEnum, lease.DEK, cmd.KeyID, kv.VersionNo,
		pol.Version, nonceBytes, cmd.Plaintext, callerAAD)
	// Zeroize DEK lease plaintext copy after use.
	defer zeroize(lease.DEK)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "seal failed", false, err)
	}

	return &EncryptResult{
		KeyID:      cmd.KeyID,
		KeyVersion: kv.VersionNo,
		SuiteID:    k.SuiteID,
		Ciphertext: envBytes,
	}, nil
}

// Decrypt performs server-side decryption per design §9.5.
func (s *Service) Decrypt(ctx context.Context, cmd DecryptCommand) (*DecryptResult, error) {
	if len(cmd.Ciphertext) > s.maxBodySize*2 {
		return nil, errorsx.New(errorsx.CodeBadRequest, "ciphertext exceeds max size", false)
	}
	env, err := envelope.Parse(cmd.Ciphertext)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeEnvelopeInvalid, "envelope invalid", false)
	}
	// Look up key by env.KeyID, scoped to tenant.
	k, err := s.store.GetKey(ctx, cmd.TenantID, string(env.KeyID))
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	if !keystate.CanDecrypt(keystate.KeyStatus(k.Status)) {
		return nil, errorsx.New(errorsx.CodeKeyDestroyed, "key destroyed", false)
	}
	// Look up exact key version (not current).
	kv, err := s.store.GetKeyVersionByNo(ctx, k.ID, env.KeyVersion)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key version not found", false)
	}
	if !keystate.KVCanDecrypt(keystate.KeyVersionStatus(kv.Status)) {
		return nil, errorsx.New(errorsx.CodeKeyDestroyed, "key version destroyed", false)
	}
	suiteEnum, _ := suiteIDFromString(k.SuiteID)

	// Issue DEK lease (decrypt path).
	lease, err := s.resolver.IssueDEKLease(ctx, kv.ID, k.ID, kv.VersionNo, suiteEnum,
		cmd.TenantID, cmd.AAD.Purpose, "")
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeTPMUnavailable, "dek lease failed", true, err)
	}
	defer zeroize(lease.DEK)

	// Rebuild caller AAD.
	callerAAD, err := envelope.CallerAADFromEnvelope(env, cmd.TenantID, cmd.AAD.Purpose, cmd.AAD.ResourceID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "aad rebuild failed", false, err)
	}

	// Open envelope.
	_, pt, err := envelope.Open(cmd.Ciphertext, lease.DEK, callerAAD)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeAADMismatch, "decrypt failed", false)
	}
	return &DecryptResult{
		KeyID:      k.ID,
		KeyVersion: kv.VersionNo,
		Plaintext:  pt,
	}, nil
}

// GenerateDataKey generates a DataKey per design §9.7 and HA-10.
func (s *Service) GenerateDataKey(ctx context.Context, cmd GenerateDataKeyCommand) (*DataKeyResult, error) {
	if cmd.TTL <= 0 {
		cmd.TTL = 5 * time.Minute
	}
	if cmd.TTL > s.dataKeyMaxTTL {
		return nil, errorsx.New(errorsx.CodeBadRequest, "ttl exceeds max", false)
	}
	if cmd.Caller != "sdk" && cmd.Caller != "direct" {
		return nil, errorsx.New(errorsx.CodeBadRequest, "caller must be sdk or direct", false)
	}
	// Quota check (P0 simple per-tenant counter).
	if !s.checkQuota(cmd.TenantID) {
		return nil, errorsx.New(errorsx.CodeRateLimited, "datakey quota exceeded", true)
	}

	k, err := s.store.GetKey(ctx, cmd.TenantID, cmd.KeyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	if !keystate.CanEncrypt(keystate.KeyStatus(k.Status)) {
		return nil, errorsx.New(errorsx.CodeKeyDisabled, "key not active", false)
	}
	suiteEnum, _ := suiteIDFromString(k.SuiteID)
	kv, err := s.store.GetCurrentKeyVersion(ctx, cmd.KeyID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "version fetch failed", false, err)
	}

	// Issue DEK lease (the KeyVersion's DEK).
	lease, err := s.resolver.IssueDEKLease(ctx, kv.ID, k.ID, kv.VersionNo, suiteEnum,
		cmd.TenantID, cmd.Purpose, "")
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeTPMUnavailable, "dek lease failed", true, err)
	}
	defer zeroize(lease.DEK)

	// Generate a fresh data key.
	dk := make([]byte, suiteEnum.KeyBytes())
	if _, err := rand.Read(dk); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "rand datakey failed", false, err)
	}
	// Wrap data key under the KeyVersion's DEK using AES-GCM.
	a, err := aead.New(suiteEnum, lease.DEK)
	if err != nil {
		zeroize(dk)
		return nil, errorsx.Wrap(errorsx.CodeInternal, "aead new failed", false, err)
	}
	nonceBytes := make([]byte, suiteEnum.NonceLen())
	if _, err := rand.Read(nonceBytes); err != nil {
		zeroize(dk)
		return nil, errorsx.Wrap(errorsx.CodeInternal, "rand nonce failed", false, err)
	}
	// DataKey envelope AAD binds tenant, key, version, purpose, context.
	encCtxHash := computeEncCtxHash(cmd.TenantID, cmd.KeyID, kv.VersionNo, cmd.Purpose, cmd.EncryptionContext)
	dkAAD := []byte("kvlt-datakey-v1|" + encCtxHash)
	ct, tag := a.Encrypt(dk, nonceBytes, dkAAD)
	// Assemble wrapped data key: nonce || ct || tag.
	wrapped := append(append(append([]byte{}, nonceBytes...), ct...), tag...)

	zeroizeBy := time.Now().Add(cmd.TTL)
	return &DataKeyResult{
		KeyID:                 cmd.KeyID,
		KeyVersion:            kv.VersionNo,
		SuiteID:               k.SuiteID,
		PlaintextDataKey:      dk,
		WrappedDataKey:        wrapped,
		ClientZeroizeBy:       zeroizeBy,
		EncryptionContextHash: encCtxHash,
	}, nil
}

// checkQuota returns false if the tenant has exceeded the per-minute quota.
func (s *Service) checkQuota(tenantID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	start, ok := s.quotaWindowStart[tenantID]
	if !ok || now.Sub(start) >= time.Minute {
		s.quotaWindowStart[tenantID] = now
		s.tenantCallCounts[tenantID] = 1
		return true
	}
	if s.tenantCallCounts[tenantID] >= s.dataKeyQuota {
		return false
	}
	s.tenantCallCounts[tenantID]++
	return true
}

// suiteIDFromString maps a suite string to aead.SuiteID.
func suiteIDFromString(s string) (aead.SuiteID, error) {
	switch s {
	case "AES_256_GCM":
		return aead.SuiteAES256GCM, nil
	case "SM4_GCM":
		return aead.SuiteSM4GCM, nil
	case "AES_256_CBC_HMAC_SHA256":
		return aead.SuiteAES256CBCHMACSHA256, nil
	case "SM4_CBC_HMAC_SM3":
		return aead.SuiteSM4CBCHMACSM3, nil
	}
	return 0, fmt.Errorf("unknown suite %s", s)
}

func computeEncCtxHash(tenantID, keyID string, keyVersion uint32, purpose string, ctx map[string]string) string {
	// Deterministic encoding: sort keys, concatenate.
	keys := make([]string, 0, len(ctx))
	for k := range ctx {
		keys = append(keys, k)
	}
	// Simple sort.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	// Build a canonical string.
	var b []byte
	b = append(b, []byte(tenantID)...)
	b = append(b, '|')
	b = append(b, []byte(keyID)...)
	b = append(b, '|')
	b = append(b, []byte(fmt.Sprintf("%d", keyVersion))...)
	b = append(b, '|')
	b = append(b, []byte(purpose)...)
	for _, k := range keys {
		b = append(b, '|')
		b = append(b, []byte(k)...)
		b = append(b, '=')
		b = append(b, []byte(ctx[k])...)
	}
	return sha256Hex(b)
}

func zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Base64Encode is a convenience for the API layer.
func Base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// Base64Decode is a convenience for the API layer.
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
