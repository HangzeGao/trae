// Package models defines the persistent data model types per design §11.
// These are shared across repository implementations.
package models

import (
	"time"
)

// Tenant is a tenant record.
type Tenant struct {
	ID           string
	Name         string
	Status       string
	CRKVersionID string // P0 reserved; may be empty
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Key is a logical key.
type Key struct {
	ID             string
	TenantID       string
	Name           string
	Purpose        string // encrypt_decrypt | datakey
	PolicyID       string
	SuiteID        string
	CurrentVersion uint32
	Status         string
	Tags           map[string]string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// KeyVersion is a version of a key with a wrapped DEK.
type KeyVersion struct {
	ID            string
	KeyID         string
	VersionNo     uint32
	SuiteID       string
	WrappedDEK    []byte // CRK-sealed; never plaintext
	WrapMetadata  []byte // JSON: crk_version, aad_digest, wrap_alg
	Status        string
	CreatedAt     time.Time
}

// CRKVersion is a CRK version metadata record.
type CRKVersion struct {
	ID         string
	Version    uint32
	Epoch      uint64 // cluster_epoch; P0 DB-maintained
	Status     string
	CreatedAt  time.Time
}

// CRKNodeEnvelope is a CRK envelope sealed for a specific node.
type CRKNodeEnvelope struct {
	ID           string
	CRKVersionID string
	NodeID       string
	Envelope     []byte // JSON-encoded provider.CRKEnvelope
	CreatedAt    time.Time
}

// Node is a registered service node.
type Node struct {
	NodeID            string
	Role              string // management | key | data
	Status            string
	ReadyReason       string // static_registration (P0) | attestation (P1+)
	ClusterEpoch      uint64
	AttestationEpoch  uint64
	Baseline          NodeBaseline
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// NodeBaseline is the host security baseline per design §6.6.
type NodeBaseline struct {
	SELinuxStatus     string
	KernelVersion     string
	VirtPlatform      string
	TPM2TSSVersion    string
	SwtpmIsolated     bool
}

// DEKLease is a DEK lease record (design §11.1).
type DEKLease struct {
	LeaseID      string
	KeyVersionID string
	NodeID       string
	TenantID     string
	Purpose      string
	SuiteID      string
	ExpiresAt    time.Time
	Revoked      bool
}

// NonceLease is a nonce counter-range lease (design §11.2).
type NonceLease struct {
	LeaseID      string
	KeyVersionID string
	NodeID       string
	Domain       uint32
	StartCounter uint64
	EndCounter   uint64
	UsedCounter  uint64
	ExpiresAt    time.Time
	Status       string // ACTIVE | RELEASED | EXPIRED | FROZEN
}

// IdempotencyKey records a processed idempotent request.
type IdempotencyKey struct {
	Key            string
	PrincipalID    string
	TenantID       string
	Method         string
	Path           string
	RequestHash    string
	ResponseStatus int
	ResponseHash   string
	CreatedAt      time.Time
}
