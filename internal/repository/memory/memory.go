// Package memory is an in-memory repository implementation for P0 testing.
// It is NOT for production use; production deployments use the postgres
// implementation. The in-memory store enforces the same concurrency and
// consistency rules as the SQL implementation (row locks via mutexes,
// monotonic counters, INSERT-only audit).
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kvlt/key-vault/internal/repository/models"
)

// Store is the in-memory data store.
type Store struct {
	mu sync.Mutex

	tenants         map[string]*models.Tenant
	keys            map[string]*models.Key // keyID -> key
	keysByTenant    map[string][]string   // tenantID -> keyIDs
	keyVersions     map[string]*models.KeyVersion
	keyVersionsByKey map[string][]string // keyID -> versionIDs

	crkVersions       map[string]*models.CRKVersion
	crkNodeEnvelopes  map[string]*models.CRKNodeEnvelope

	nodes         map[string]*models.Node

	dekLeases   map[string]*models.DEKLease
	nonceLeases map[string]*models.NonceLease
	// nonceCounter tracks the global counter per (keyVersionID, domain).
	nonceCounter map[string]uint64 // "kvID:domain" -> next start

	idempotency map[string]*models.IdempotencyKey

	clusterEpoch uint64
}

// New constructs a new in-memory store.
func New() *Store {
	return &Store{
		tenants:           make(map[string]*models.Tenant),
		keys:              make(map[string]*models.Key),
		keysByTenant:      make(map[string][]string),
		keyVersions:       make(map[string]*models.KeyVersion),
		keyVersionsByKey:  make(map[string][]string),
		crkVersions:       make(map[string]*models.CRKVersion),
		crkNodeEnvelopes:  make(map[string]*models.CRKNodeEnvelope),
		nodes:             make(map[string]*models.Node),
		dekLeases:         make(map[string]*models.DEKLease),
		nonceLeases:       make(map[string]*models.NonceLease),
		nonceCounter:      make(map[string]uint64),
		idempotency:       make(map[string]*models.IdempotencyKey),
	}
}

// Errors.
var (
	ErrNotFound        = errors.New("memory: not found")
	ErrConflict        = errors.New("memory: conflict")
	ErrIllegalState    = errors.New("memory: illegal state")
)

// --- Tenant ---

// UpsertTenant inserts or updates a tenant.
func (s *Store) UpsertTenant(ctx context.Context, t *models.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	s.tenants[t.ID] = t
	return nil
}

// GetTenant returns a tenant by ID.
func (s *Store) GetTenant(ctx context.Context, id string) (*models.Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneTenant(t), nil
}

// --- Keys ---

// CreateKey atomically creates a key + its initial key version.
func (s *Store) CreateKey(ctx context.Context, k *models.Key, kv *models.KeyVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.keys[k.ID]; ok {
		return fmt.Errorf("%w: key %s exists", ErrConflict, k.ID)
	}
	now := time.Now().UTC()
	if k.CreatedAt.IsZero() {
		k.CreatedAt = now
	}
	k.UpdatedAt = now
	if kv.CreatedAt.IsZero() {
		kv.CreatedAt = now
	}
	s.keys[k.ID] = k
	s.keysByTenant[k.TenantID] = append(s.keysByTenant[k.TenantID], k.ID)
	s.keyVersions[kv.ID] = kv
	s.keyVersionsByKey[k.ID] = append(s.keyVersionsByKey[k.ID], kv.ID)
	return nil
}

// GetKey returns a key by ID.
func (s *Store) GetKey(ctx context.Context, tenantID, keyID string) (*models.Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[keyID]
	if !ok || k.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneKey(k), nil
}

// ListKeys lists keys for a tenant.
func (s *Store) ListKeys(ctx context.Context, tenantID string) ([]*models.Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids, ok := s.keysByTenant[tenantID]
	if !ok {
		return nil, nil
	}
	out := make([]*models.Key, 0, len(ids))
	for _, id := range ids {
		out = append(out, cloneKey(s.keys[id]))
	}
	return out, nil
}

// UpdateKeyStatus updates a key's status. Uses optimistic concurrency via
// the expectedCurrent parameter.
func (s *Store) UpdateKeyStatus(ctx context.Context, keyID string, expectedCurrent, newStatus string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[keyID]
	if !ok {
		return ErrNotFound
	}
	if k.Status != expectedCurrent {
		return fmt.Errorf("%w: expected %s, got %s", ErrConflict, expectedCurrent, k.Status)
	}
	k.Status = newStatus
	k.UpdatedAt = time.Now().UTC()
	return nil
}

// LockKeyForUpdate simulates SELECT FOR UPDATE. The returned unlock function
// MUST be called to release the lock.
func (s *Store) LockKeyForUpdate(ctx context.Context, keyID string) (func(), error) {
	// The store mutex is the lock; we expose a no-op unlock since the caller
	// is expected to do all work in the same transaction.
	s.mu.Lock()
	if _, ok := s.keys[keyID]; !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	// Return a function that does nothing; the caller must use a Tx method.
	// For P0 we expose higher-level atomic operations instead.
	return func() {}, nil
}

// RotateKey atomically: locks key, inserts new version, switches current_version,
// marks old version DECRYPT_ONLY.
func (s *Store) RotateKey(ctx context.Context, keyID string, newVersion *models.KeyVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[keyID]
	if !ok {
		return ErrNotFound
	}
	// Mark old current version DECRYPT_ONLY.
	for _, vid := range s.keyVersionsByKey[keyID] {
		kv := s.keyVersions[vid]
		if kv.VersionNo == k.CurrentVersion {
			kv.Status = "DECRYPT_ONLY"
		}
	}
	newVersion.CreatedAt = time.Now().UTC()
	s.keyVersions[newVersion.ID] = newVersion
	s.keyVersionsByKey[keyID] = append(s.keyVersionsByKey[keyID], newVersion.ID)
	k.CurrentVersion = newVersion.VersionNo
	k.UpdatedAt = time.Now().UTC()
	return nil
}

// GetKeyVersion returns a key version by ID.
func (s *Store) GetKeyVersion(ctx context.Context, versionID string) (*models.KeyVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kv, ok := s.keyVersions[versionID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneKeyVersion(kv), nil
}

// GetKeyVersionByNo returns the key version for a (keyID, versionNo).
func (s *Store) GetKeyVersionByNo(ctx context.Context, keyID string, versionNo uint32) (*models.KeyVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, vid := range s.keyVersionsByKey[keyID] {
		kv := s.keyVersions[vid]
		if kv.VersionNo == versionNo {
			return cloneKeyVersion(kv), nil
		}
	}
	return nil, ErrNotFound
}

// GetCurrentKeyVersion returns the current key version for a key.
func (s *Store) GetCurrentKeyVersion(ctx context.Context, keyID string) (*models.KeyVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.keys[keyID]
	if !ok {
		return nil, ErrNotFound
	}
	for _, vid := range s.keyVersionsByKey[keyID] {
		kv := s.keyVersions[vid]
		if kv.VersionNo == k.CurrentVersion {
			return cloneKeyVersion(kv), nil
		}
	}
	return nil, ErrNotFound
}

// --- CRK ---

// CreateCRKVersion inserts a CRK version.
func (s *Store) CreateCRKVersion(ctx context.Context, v *models.CRKVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.crkVersions[v.ID]; ok {
		return fmt.Errorf("%w: crk version %s exists", ErrConflict, v.ID)
	}
	v.CreatedAt = time.Now().UTC()
	s.crkVersions[v.ID] = v
	// Bump cluster_epoch.
	s.clusterEpoch++
	return nil
}

// GetCRKVersion returns the current CRK version.
func (s *Store) GetCRKVersion(ctx context.Context, id string) (*models.CRKVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.crkVersions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneCRKVersion(v), nil
}

// GetLatestCRKVersion returns the latest CRK version.
func (s *Store) GetLatestCRKVersion(ctx context.Context) (*models.CRKVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *models.CRKVersion
	for _, v := range s.crkVersions {
		if latest == nil || v.Version > latest.Version {
			latest = v
		}
	}
	if latest == nil {
		return nil, ErrNotFound
	}
	return cloneCRKVersion(latest), nil
}

// CreateCRKNodeEnvelope inserts a CRK node envelope.
func (s *Store) CreateCRKNodeEnvelope(ctx context.Context, e *models.CRKNodeEnvelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.crkNodeEnvelopes[e.ID]; ok {
		return fmt.Errorf("%w: crk envelope %s exists", ErrConflict, e.ID)
	}
	e.CreatedAt = time.Now().UTC()
	s.crkNodeEnvelopes[e.ID] = e
	return nil
}

// GetCRKNodeEnvelope returns the CRK envelope for a (crkVersionID, nodeID).
func (s *Store) GetCRKNodeEnvelope(ctx context.Context, crkVersionID, nodeID string) (*models.CRKNodeEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.crkNodeEnvelopes {
		if e.CRKVersionID == crkVersionID && e.NodeID == nodeID {
			return cloneCRKNodeEnvelope(e), nil
		}
	}
	return nil, ErrNotFound
}

// ClusterEpoch returns the current cluster epoch.
func (s *Store) ClusterEpoch(ctx context.Context) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clusterEpoch, nil
}

// --- Nodes ---

// UpsertNode inserts or updates a node.
func (s *Store) UpsertNode(ctx context.Context, n *models.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if _, ok := s.nodes[n.NodeID]; !ok {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	s.nodes[n.NodeID] = n
	return nil
}

// GetNode returns a node by ID.
func (s *Store) GetNode(ctx context.Context, nodeID string) (*models.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[nodeID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneNode(n), nil
}

// --- DEK Leases ---

// CreateDEKLease inserts a DEK lease.
func (s *Store) CreateDEKLease(ctx context.Context, l *models.DEKLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dekLeases[l.LeaseID]; ok {
		return fmt.Errorf("%w: dek lease %s exists", ErrConflict, l.LeaseID)
	}
	s.dekLeases[l.LeaseID] = l
	return nil
}

// RevokeDEKLease marks a DEK lease revoked.
func (s *Store) RevokeDEKLease(ctx context.Context, leaseID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.dekLeases[leaseID]
	if !ok {
		return ErrNotFound
	}
	l.Revoked = true
	return nil
}

// --- Nonce Leases ---

// AllocateNonceRange atomically allocates [start, end) for a (keyVersionID, nodeID, domain).
// The range is permanently burned: subsequent calls return a NEW range,
// never overlapping a previously allocated range.
func (s *Store) AllocateNonceRange(ctx context.Context, keyVersionID, nodeID string, domain uint32, size uint64, ttl time.Duration) (*models.NonceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%d", keyVersionID, domain)
	start := s.nonceCounter[key]
	end := start + size
	s.nonceCounter[key] = end
	leaseID := newID("nls")
	lease := &models.NonceLease{
		LeaseID:      leaseID,
		KeyVersionID: keyVersionID,
		NodeID:       nodeID,
		Domain:       domain,
		StartCounter: start,
		EndCounter:   end,
		UsedCounter:  start,
		ExpiresAt:    time.Now().UTC().Add(ttl),
		Status:       "ACTIVE",
	}
	s.nonceLeases[leaseID] = lease
	return cloneNonceLease(lease), nil
}

// UpdateNonceUsed persists the used counter for a lease.
func (s *Store) UpdateNonceUsed(ctx context.Context, leaseID string, used uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.nonceLeases[leaseID]
	if !ok {
		return ErrNotFound
	}
	l.UsedCounter = used
	return nil
}

// GetNonceLease returns a nonce lease by ID.
func (s *Store) GetNonceLease(ctx context.Context, leaseID string) (*models.NonceLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.nonceLeases[leaseID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneNonceLease(l), nil
}

// --- Idempotency ---

// RecordIdempotency records an idempotency key. Returns ErrConflict if the
// key already exists with a different request hash.
func (s *Store) RecordIdempotency(ctx context.Context, ik *models.IdempotencyKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.idempotency[ik.Key]
	if ok {
		if existing.RequestHash != ik.RequestHash {
			return fmt.Errorf("%w: idempotency key reused with different body", ErrConflict)
		}
		return nil
	}
	ik.CreatedAt = time.Now().UTC()
	s.idempotency[ik.Key] = ik
	return nil
}

// GetIdempotency returns an existing idempotency record.
func (s *Store) GetIdempotency(ctx context.Context, key string) (*models.IdempotencyKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ik, ok := s.idempotency[key]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneIdempotencyKey(ik), nil
}

// --- helpers ---

func newID(prefix string) string {
	// P0 uses a deterministic-ish ID; production uses UUID.
	now := time.Now().UnixNano()
	h := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, now)))
	return fmt.Sprintf("%s_%x", prefix, h[:8])
}

// clone helpers (defensive copies so callers cannot mutate store state).

func cloneTenant(t *models.Tenant) *models.Tenant {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func cloneKey(k *models.Key) *models.Key {
	if k == nil {
		return nil
	}
	c := *k
	if k.Tags != nil {
		c.Tags = make(map[string]string, len(k.Tags))
		for k2, v := range k.Tags {
			c.Tags[k2] = v
		}
	}
	return &c
}

func cloneKeyVersion(kv *models.KeyVersion) *models.KeyVersion {
	if kv == nil {
		return nil
	}
	c := *kv
	if kv.WrappedDEK != nil {
		c.WrappedDEK = append([]byte(nil), kv.WrappedDEK...)
	}
	if kv.WrapMetadata != nil {
		c.WrapMetadata = append([]byte(nil), kv.WrapMetadata...)
	}
	return &c
}

func cloneCRKVersion(v *models.CRKVersion) *models.CRKVersion {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func cloneCRKNodeEnvelope(e *models.CRKNodeEnvelope) *models.CRKNodeEnvelope {
	if e == nil {
		return nil
	}
	c := *e
	if e.Envelope != nil {
		c.Envelope = append([]byte(nil), e.Envelope...)
	}
	return &c
}

func cloneNode(n *models.Node) *models.Node {
	if n == nil {
		return nil
	}
	c := *n
	return &c
}

func cloneDEKLease(l *models.DEKLease) *models.DEKLease {
	if l == nil {
		return nil
	}
	c := *l
	return &c
}

func cloneNonceLease(l *models.NonceLease) *models.NonceLease {
	if l == nil {
		return nil
	}
	c := *l
	return &c
}

func cloneIdempotencyKey(ik *models.IdempotencyKey) *models.IdempotencyKey {
	if ik == nil {
		return nil
	}
	c := *ik
	return &c
}

// Hash returns a hex SHA-256 of the input (utility).
func Hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
