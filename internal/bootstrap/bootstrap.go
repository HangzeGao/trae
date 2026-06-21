// Package bootstrap wires all components together for the P0 service.
package bootstrap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kvlt/key-vault/internal/api/admin"
	cryptoapi "github.com/kvlt/key-vault/internal/api/crypto"
	"github.com/kvlt/key-vault/internal/application/crypto"
	"github.com/kvlt/key-vault/internal/application/keys"
	"github.com/kvlt/key-vault/internal/application/nodes"
	"github.com/kvlt/key-vault/internal/auth/hmacsign"
	"github.com/kvlt/key-vault/internal/auth/jwt"
	"github.com/kvlt/key-vault/internal/auth/principal"
	"github.com/kvlt/key-vault/internal/config"
	"github.com/kvlt/key-vault/internal/crypto/aad"
	"github.com/kvlt/key-vault/internal/crypto/nonce"
	"github.com/kvlt/key-vault/internal/domain/policy"
	"github.com/kvlt/key-vault/internal/repository/memory"
	"github.com/kvlt/key-vault/internal/repository/models"
	"github.com/kvlt/key-vault/internal/resolver/keyresolver"
	"github.com/kvlt/key-vault/internal/tpm/provider"
)

// App is the assembled application.
type App struct {
	Cfg           *config.Config
	Store         *memory.Store
	TPM           provider.Provider
	Resolver      *keyresolver.Resolver
	Policies      *policy.Engine
	KeyService    *keys.Service
	CryptoService *crypto.Service
	NodeService   *nodes.Service
	NonceManager  *nonce.Manager
	JWTVerifier   *jwt.Verifier
	HMACVerifier  *hmacsign.Verifier
	StaticTokens  map[string]*principal.Principal
}

// Build assembles the application from config.
func Build(ctx context.Context, cfg *config.Config) (*App, error) {
	// 1. Repository.
	var store *memory.Store
	if cfg.Database.Driver == "memory" {
		store = memory.New()
	} else {
		return nil, fmt.Errorf("bootstrap: database driver %s not implemented in P0 (use 'memory')", cfg.Database.Driver)
	}

	// 2. TPM provider.
	var tpm provider.Provider
	var err error
	switch cfg.TPM.Provider {
	case "swtpm", "software":
		tpm, err = provider.NewSoftwareProvider(cfg.TPM.StateDir)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: tpm: %w", err)
		}
	default:
		return nil, fmt.Errorf("bootstrap: unknown tpm provider %s", cfg.TPM.Provider)
	}

	// 3. Policy engine.
	polEng := policy.NewEngine()
	defPol := policy.DefaultPolicy()
	if err := polEng.Load(defPol); err != nil {
		return nil, fmt.Errorf("bootstrap: policy: %w", err)
	}

	// 4. Resolver.
	resolver := keyresolver.New(tpm, "kvlt-nrwk-v1", 5*time.Minute)

	// 5. Nonce manager. The allocator delegates to the store.
	alloc := &storeNonceAllocator{store: store}
	nonceMgr := nonce.NewManager(alloc, cfg.Nonce.LeaseSize,
		cfg.Nonce.PrefetchWatermark, cfg.Nonce.ThrottleWatermark, cfg.Nonce.LeaseTTL)

	// 6. Application services.
	keySvc := keys.New(store, resolver, polEng)
	cryptoSvc := crypto.New(store, resolver, polEng, nonceMgr,
		cfg.Server.MaxRequestBody, cfg.DataKey.MaxTTL, cfg.DataKey.QuotaPerMin)
	baselineChecker := &nodes.DefaultBaselineChecker{
		SELinuxRequired: cfg.Baseline.SELinuxRequired,
	}
	nodeSvc := nodes.New(store, baselineChecker)

	// 7. Auth verifiers.
	jwtVerifier := jwt.NewVerifier(cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.JWTAlgWhite)
	hmacVerifier := hmacsign.NewVerifier(cfg.Auth.HMACMaxSkew)

	// 8. Bootstrap: create default tenant, NRWK, CRK, CRK envelope.
	if err := bootstrapCluster(ctx, cfg, store, tpm, resolver); err != nil {
		return nil, fmt.Errorf("bootstrap: cluster: %w", err)
	}

	// Wire resolver's fetch hook to the store.
	resolver.SetFetchDEKHook(func(ctx context.Context, keyVersionID string) ([]byte, []byte, error) {
		// We need to find the key version by ID. The store has GetKeyVersion.
		kv, err := store.GetKeyVersion(ctx, keyVersionID)
		if err != nil {
			return nil, nil, err
		}
		return kv.WrappedDEK, kv.WrapMetadata, nil
	})

	return &App{
		Cfg:           cfg,
		Store:         store,
		TPM:           tpm,
		Resolver:      resolver,
		Policies:      polEng,
		KeyService:    keySvc,
		CryptoService: cryptoSvc,
		NodeService:   nodeSvc,
		NonceManager:  nonceMgr,
		JWTVerifier:   jwtVerifier,
		HMACVerifier:  hmacVerifier,
		StaticTokens:  loadStaticTokens(),
	}, nil
}

// loadStaticTokens reads static token → principal mappings from the
// KVLT_STATIC_TOKENS env var (JSON array of {token, principal} objects).
// This is a P0 convenience for development/demo. In production, use JWT/HMAC.
func loadStaticTokens() map[string]*principal.Principal {
	tokens := make(map[string]*principal.Principal)
	raw := os.Getenv("KVLT_STATIC_TOKENS")
	if raw == "" {
		return tokens
	}
	var entries []struct {
		Token    string   `json:"token"`
		TenantID string   `json:"tenant_id"`
		Scopes   []string `json:"scopes"`
		Roles    []string `json:"roles"`
		Plane    string   `json:"plane"`
		NodeID   string   `json:"node_id"`
	}
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return tokens
	}
	for _, e := range entries {
		plane := principal.PlaneManagement
		if e.Plane == "data" {
			plane = principal.PlaneData
		}
		tokens[e.Token] = &principal.Principal{
			ID:       "static:" + e.Token[:8],
			TenantID: e.TenantID,
			Scopes:   e.Scopes,
			Roles:    e.Roles,
			Plane:    plane,
			NodeID:   e.NodeID,
		}
	}
	return tokens
}

// bootstrapCluster creates the default tenant, NRWK, CRK, and CRK envelope.
func bootstrapCluster(ctx context.Context, cfg *config.Config, store *memory.Store,
	tpm provider.Provider, resolver *keyresolver.Resolver) error {
	// Default tenant.
	tenant := &models.Tenant{
		ID:     "t-default",
		Name:   "default",
		Status: "active",
	}
	if err := store.UpsertTenant(ctx, tenant); err != nil {
		return err
	}

	// Initialize resolver (loads/creates NRWK).
	baseline := hashBaseline(cfg.Baseline)
	policyDigest := sha256.Sum256([]byte("kvlt-default-policy-v1"))
	if err := resolver.Init(ctx, "kvlt-cluster-v1", "node-bootstrap", "management", baseline, policyDigest[:]); err != nil {
		return err
	}

	// Create CRK version if none exists.
	if _, err := store.GetLatestCRKVersion(ctx); err == nil {
		// Already bootstrapped.
		return nil
	} else if !errors.Is(err, memory.ErrNotFound) {
		return err
	}

	// Generate CRK plaintext (32 bytes).
	crk := make([]byte, 32)
	if _, err := rand.Read(crk); err != nil {
		return fmt.Errorf("bootstrap: rand crk: %w", err)
	}

	// Create CRK version record.
	crkVer := &models.CRKVersion{
		ID:        "crk-v1",
		Version:   1,
		Epoch:     1,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateCRKVersion(ctx, crkVer); err != nil {
		return err
	}

	// Seal CRK under NRWK.
	nrwk, err := tpm.EnsureNRWK(ctx, "kvlt-nrwk-v1")
	if err != nil {
		return err
	}
	a := aad.CRKAAD{
		ClusterID:      "kvlt-cluster-v1",
		NodeID:         "node-bootstrap",
		PlaneRole:      "management",
		CRKVersion:     1,
		NRWKName:       "kvlt-nrwk-v1",
		BaselineDigest: baseline,
		PolicyDigest:   policyDigest[:],
	}
	env, err := tpm.SealCRK(ctx, nrwk, crk, a)
	if err != nil {
		return fmt.Errorf("bootstrap: seal crk: %w", err)
	}

	// Persist CRK envelope (JSON-encoded).
	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}
	crkNodeEnv := &models.CRKNodeEnvelope{
		ID:           "crkenv-bootstrap",
		CRKVersionID: crkVer.ID,
		NodeID:       "node-bootstrap",
		Envelope:     envBytes,
	}
	if err := store.CreateCRKNodeEnvelope(ctx, crkNodeEnv); err != nil {
		return err
	}

	// Cache envelope in resolver.
	resolver.SetCRKEnvelope(env)
	return nil
}

// storeNonceAllocator adapts the memory store to the nonce.Allocator interface.
type storeNonceAllocator struct {
	store *memory.Store
}

func (a *storeNonceAllocator) AllocateRange(keyVersionID, nodeID string, domain uint32, size uint64, ttl time.Duration) (*nonce.Lease, error) {
	ctx := context.Background()
	nl, err := a.store.AllocateNonceRange(ctx, keyVersionID, nodeID, domain, size, ttl)
	if err != nil {
		return nil, err
	}
	return &nonce.Lease{
		LeaseID:      nl.LeaseID,
		KeyVersionID: nl.KeyVersionID,
		NodeID:       nl.NodeID,
		Domain:       nl.Domain,
		StartCounter: nl.StartCounter,
		EndCounter:   nl.EndCounter,
		UsedCounter:  nl.UsedCounter,
		ExpiresAt:    nl.ExpiresAt,
		Status:       nonce.LeaseStatus(nl.Status),
	}, nil
}

func (a *storeNonceAllocator) UpdateUsed(leaseID string, used uint64) error {
	return a.store.UpdateNonceUsed(context.Background(), leaseID, used)
}

func (a *storeNonceAllocator) GetLease(leaseID string) (*nonce.Lease, error) {
	nl, err := a.store.GetNonceLease(context.Background(), leaseID)
	if err != nil {
		return nil, err
	}
	return &nonce.Lease{
		LeaseID:      nl.LeaseID,
		KeyVersionID: nl.KeyVersionID,
		NodeID:       nl.NodeID,
		Domain:       nl.Domain,
		StartCounter: nl.StartCounter,
		EndCounter:   nl.EndCounter,
		UsedCounter:  nl.UsedCounter,
		ExpiresAt:    nl.ExpiresAt,
		Status:       nonce.LeaseStatus(nl.Status),
	}, nil
}

func hashBaseline(b config.BaselineConfig) []byte {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", b)))
	return h.Sum(nil)
}

// hexEncode is a small helper retained for diagnostic output.
func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// _ unused import guard.
var (
	_ = admin.New
	_ = cryptoapi.New
	_ = json.Marshal
)
