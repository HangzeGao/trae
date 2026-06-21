// Package keys implements the Key application service per design §9.3, §9.6.
package keys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kvlt/key-vault/internal/crypto/aead"
	keystate "github.com/kvlt/key-vault/internal/domain/key/state"
	"github.com/kvlt/key-vault/internal/domain/policy"
	"github.com/kvlt/key-vault/internal/errorsx"
	"github.com/kvlt/key-vault/internal/repository/models"
	"github.com/kvlt/key-vault/internal/resolver/keyresolver"
)

// CreateKeyCommand is the input for CreateKey.
type CreateKeyCommand struct {
	TenantID    string
	Name        string
	Purpose     string
	PolicyID    string
	SuiteID     string
	Tags        map[string]string
	IdempotencyKey string
	PrincipalID string
}

// KeyDTO is the public representation of a key. Never includes wrapped_dek.
type KeyDTO struct {
	KeyID          string            `json:"key_id"`
	TenantID       string            `json:"tenant_id"`
	Name           string            `json:"name"`
	Purpose        string            `json:"purpose"`
	PolicyID       string            `json:"policy_id"`
	SuiteID        string            `json:"suite_id"`
	CurrentVersion uint32            `json:"current_version"`
	Status         string            `json:"status"`
	Tags           map[string]string `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// RotateKeyCommand is the input for RotateKey.
type RotateKeyCommand struct {
	TenantID string
	KeyID    string
	PrincipalID string
}

// Service is the key application service.
type Service struct {
	mu        sync.Mutex
	store     Store
	resolver  *keyresolver.Resolver
	policies  *policy.Engine
}

// Store is the subset of repository methods used by the key service.
type Store interface {
	GetTenant(ctx context.Context, id string) (*models.Tenant, error)
	CreateKey(ctx context.Context, k *models.Key, kv *models.KeyVersion) error
	GetKey(ctx context.Context, tenantID, keyID string) (*models.Key, error)
	ListKeys(ctx context.Context, tenantID string) ([]*models.Key, error)
	UpdateKeyStatus(ctx context.Context, keyID, expectedCurrent, newStatus string) error
	RotateKey(ctx context.Context, keyID string, newVersion *models.KeyVersion) error
	GetCurrentKeyVersion(ctx context.Context, keyID string) (*models.KeyVersion, error)
	GetKeyVersionByNo(ctx context.Context, keyID string, versionNo uint32) (*models.KeyVersion, error)
}

// New constructs a key service.
func New(store Store, resolver *keyresolver.Resolver, policies *policy.Engine) *Service {
	return &Service{store: store, resolver: resolver, policies: policies}
}

// CreateKey creates a new key + initial key version. Per design §9.3:
//   - DEK generated server-side and sealed under CRK.
//   - Database stores only wrapped DEK.
//   - Response contains no DEK material.
func (s *Service) CreateKey(ctx context.Context, cmd CreateKeyCommand) (*KeyDTO, error) {
	if cmd.TenantID == "" || cmd.Name == "" || cmd.Purpose == "" {
		return nil, errorsx.New(errorsx.CodeInvalidArgument, "missing required field", false)
	}
	if cmd.Purpose != "encrypt_decrypt" && cmd.Purpose != "datakey" {
		return nil, errorsx.New(errorsx.CodeInvalidArgument, "invalid purpose", false)
	}
	// Validate policy + suite.
	pol, err := s.policies.Get(cmd.PolicyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "unknown policy", false)
	}
	suite, err := pol.SuiteByID(cmd.SuiteID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "unknown suite", false)
	}
	if !policy.CanEncrypt(suite.Status) {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "suite not available for encryption", false)
	}
	if suite.Mode != policy.ModeGCM {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "only GCM suites allowed for new encryption in P0", false)
	}

	// Validate tenant exists.
	if _, err := s.store.GetTenant(ctx, cmd.TenantID); err != nil {
		return nil, errorsx.New(errorsx.CodePermissionDenied, "tenant not found", false)
	}

	suiteEnum, err := suiteIDFromString(cmd.SuiteID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodePolicyDenied, "invalid suite_id", false)
	}

	keyID := newID("key")
	versionID := newID("kv")

	// Generate and wrap DEK via resolver (CRK critical section).
	dm, err := s.resolver.GenerateAndWrapDEK(ctx, suiteEnum, versionID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeTPMUnavailable, "dek generation failed", true, err)
	}

	now := time.Now().UTC()
	k := &models.Key{
		ID:             keyID,
		TenantID:       cmd.TenantID,
		Name:           cmd.Name,
		Purpose:        cmd.Purpose,
		PolicyID:       cmd.PolicyID,
		SuiteID:        cmd.SuiteID,
		CurrentVersion: 1,
		Status:         string(keystate.KeyActive),
		Tags:           cmd.Tags,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	kv := &models.KeyVersion{
		ID:           versionID,
		KeyID:        keyID,
		VersionNo:    1,
		SuiteID:      cmd.SuiteID,
		WrappedDEK:   dm.WrappedDEK,
		WrapMetadata: dm.WrapMetadata,
		Status:       string(keystate.KVActive),
		CreatedAt:    now,
	}
	if err := s.store.CreateKey(ctx, k, kv); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeDBConflict, "create key failed", true, err)
	}
	return toDTO(k), nil
}

// GetKey returns a key by ID. Cross-tenant access returns PermissionDenied.
func (s *Service) GetKey(ctx context.Context, tenantID, keyID string) (*KeyDTO, error) {
	k, err := s.store.GetKey(ctx, tenantID, keyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	return toDTO(k), nil
}

// ListKeys lists keys for a tenant.
func (s *Service) ListKeys(ctx context.Context, tenantID string) ([]*KeyDTO, error) {
	ks, err := s.store.ListKeys(ctx, tenantID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "list keys failed", false, err)
	}
	out := make([]*KeyDTO, 0, len(ks))
	for _, k := range ks {
		out = append(out, toDTO(k))
	}
	return out, nil
}

// DisableKey transitions a key from ACTIVE to DISABLED.
func (s *Service) DisableKey(ctx context.Context, tenantID, keyID, principalID string) error {
	k, err := s.store.GetKey(ctx, tenantID, keyID)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	newStatus, err := keystate.TransitionKey(keystate.KeyStatus(k.Status), keystate.EvDisable)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyDisabled, "illegal state transition", false)
	}
	if err := s.store.UpdateKeyStatus(ctx, keyID, k.Status, string(newStatus)); err != nil {
		return errorsx.Wrap(errorsx.CodeDBConflict, "update failed", true, err)
	}
	return nil
}

// EnableKey transitions a key from DISABLED to ACTIVE.
func (s *Service) EnableKey(ctx context.Context, tenantID, keyID, principalID string) error {
	k, err := s.store.GetKey(ctx, tenantID, keyID)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	newStatus, err := keystate.TransitionKey(keystate.KeyStatus(k.Status), keystate.EvEnable)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyDisabled, "illegal state transition", false)
	}
	if err := s.store.UpdateKeyStatus(ctx, keyID, k.Status, string(newStatus)); err != nil {
		return errorsx.Wrap(errorsx.CodeDBConflict, "update failed", true, err)
	}
	return nil
}

// ScheduleDestroy transitions a key to DESTROY_PENDING.
func (s *Service) ScheduleDestroy(ctx context.Context, tenantID, keyID, principalID string) error {
	k, err := s.store.GetKey(ctx, tenantID, keyID)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	newStatus, err := keystate.TransitionKey(keystate.KeyStatus(k.Status), keystate.EvScheduleDestroy)
	if err != nil {
		return errorsx.New(errorsx.CodeKeyDisabled, "illegal state transition", false)
	}
	if err := s.store.UpdateKeyStatus(ctx, keyID, k.Status, string(newStatus)); err != nil {
		return errorsx.Wrap(errorsx.CodeDBConflict, "update failed", true, err)
	}
	return nil
}

// RotateKey creates a new key version and atomically switches current_version.
// Per design §9.6: old version becomes DECRYPT_ONLY; new version is ACTIVE.
func (s *Service) RotateKey(ctx context.Context, cmd RotateKeyCommand) (*KeyDTO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, err := s.store.GetKey(ctx, cmd.TenantID, cmd.KeyID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeKeyNotFound, "key not found", false)
	}
	if k.Status != string(keystate.KeyActive) {
		return nil, errorsx.New(errorsx.CodeKeyDisabled, "key not active", false)
	}
	suiteEnum, err := suiteIDFromString(k.SuiteID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "bad suite", false, err)
	}
	newVersionNo := k.CurrentVersion + 1
	newVersionID := newID("kv")
	dm, err := s.resolver.GenerateAndWrapDEK(ctx, suiteEnum, newVersionID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeTPMUnavailable, "dek generation failed", true, err)
	}
	newKV := &models.KeyVersion{
		ID:           newVersionID,
		KeyID:        cmd.KeyID,
		VersionNo:    newVersionNo,
		SuiteID:      k.SuiteID,
		WrappedDEK:   dm.WrappedDEK,
		WrapMetadata: dm.WrapMetadata,
		Status:       string(keystate.KVActive),
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.RotateKey(ctx, cmd.KeyID, newKV); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeDBConflict, "rotate failed", true, err)
	}
	// Re-fetch to return updated DTO.
	k2, err := s.store.GetKey(ctx, cmd.TenantID, cmd.KeyID)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "re-fetch failed", false, err)
	}
	return toDTO(k2), nil
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

func toDTO(k *models.Key) *KeyDTO {
	return &KeyDTO{
		KeyID:          k.ID,
		TenantID:       k.TenantID,
		Name:           k.Name,
		Purpose:        k.Purpose,
		PolicyID:       k.PolicyID,
		SuiteID:        k.SuiteID,
		CurrentVersion: k.CurrentVersion,
		Status:         k.Status,
		Tags:           k.Tags,
		CreatedAt:      k.CreatedAt,
	}
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// ErrInternal is a sentinel for unexpected errors.
var ErrInternal = errors.New("keys: internal")
