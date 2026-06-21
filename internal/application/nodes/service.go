// Package nodes implements the Node application service per design §9.1, §6.6.
package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	nodestate "github.com/kvlt/key-vault/internal/domain/node/state"
	"github.com/kvlt/key-vault/internal/errorsx"
	"github.com/kvlt/key-vault/internal/repository/models"
)

// RegisterCommand is the input for Register.
type RegisterCommand struct {
	NodeID    string
	Role      string // management | key | data
	Baseline  models.NodeBaseline
	PrincipalID string
}

// NodeDTO is the public representation of a node.
type NodeDTO struct {
	NodeID           string    `json:"node_id"`
	Role             string    `json:"role"`
	Status           string    `json:"status"`
	ReadyReason      string    `json:"ready_reason"`
	ClusterEpoch     uint64    `json:"cluster_epoch"`
	AttestationEpoch uint64    `json:"attestation_epoch"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Store is the repository subset used by the node service.
type Store interface {
	UpsertNode(ctx context.Context, n *models.Node) error
	GetNode(ctx context.Context, nodeID string) (*models.Node, error)
	ClusterEpoch(ctx context.Context) (uint64, error)
}

// BaselineChecker validates host security baselines per design §6.6.
type BaselineChecker interface {
	Validate(b models.NodeBaseline) error
}

// Service is the node application service.
type Service struct {
	store    Store
	checker  BaselineChecker
}

// New constructs a node service.
func New(store Store, checker BaselineChecker) *Service {
	return &Service{store: store, checker: checker}
}

// Register registers a node. Per design §9.1, P0 uses static registration.
// The node enters REGISTERED status; transition to READY requires a separate
// call (e.g. heartbeat) that validates baseline.
func (s *Service) Register(ctx context.Context, cmd RegisterCommand) (*NodeDTO, error) {
	if cmd.NodeID == "" || cmd.Role == "" {
		return nil, errorsx.New(errorsx.CodeInvalidArgument, "missing node_id or role", false)
	}
	if cmd.Role != "management" && cmd.Role != "key" && cmd.Role != "data" {
		return nil, errorsx.New(errorsx.CodeInvalidArgument, "invalid role", false)
	}
	// Validate baseline (HA-08 / §6.6).
	if err := s.checker.Validate(cmd.Baseline); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeBaselineCheckFailed, "baseline check failed", false, err)
	}
	epoch, err := s.store.ClusterEpoch(ctx)
	if err != nil {
		return nil, errorsx.Wrap(errorsx.CodeInternal, "epoch fetch failed", false, err)
	}
	n := &models.Node{
		NodeID:           cmd.NodeID,
		Role:             cmd.Role,
		Status:           string(nodestate.StatusRegistered),
		ReadyReason:      "static_registration",
		ClusterEpoch:     epoch,
		AttestationEpoch: epoch,
		Baseline:         cmd.Baseline,
	}
	if err := s.store.UpsertNode(ctx, n); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeDBConflict, "upsert failed", true, err)
	}
	return toDTO(n), nil
}

// MarkReady transitions a node from REGISTERED to READY (P0 static policy).
func (s *Service) MarkReady(ctx context.Context, nodeID, principalID string) (*NodeDTO, error) {
	n, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeNodeNotReady, "node not found", false)
	}
	newStatus, err := nodestate.Transition(nodestate.Status(n.Status), nodestate.EvAuthOrAttestationPass)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeNodeNotReady, "illegal transition", false)
	}
	n.Status = string(newStatus)
	if err := s.store.UpsertNode(ctx, n); err != nil {
		return nil, errorsx.Wrap(errorsx.CodeDBConflict, "upsert failed", true, err)
	}
	return toDTO(n), nil
}

// Revoke transitions a node to REVOKED.
func (s *Service) Revoke(ctx context.Context, nodeID, principalID string) error {
	n, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return errorsx.New(errorsx.CodeNodeNotReady, "node not found", false)
	}
	newStatus, err := nodestate.Transition(nodestate.Status(n.Status), nodestate.EvRevoke)
	if err != nil {
		return errorsx.New(errorsx.CodeNodeNotReady, "illegal transition", false)
	}
	n.Status = string(newStatus)
	if err := s.store.UpsertNode(ctx, n); err != nil {
		return errorsx.Wrap(errorsx.CodeDBConflict, "upsert failed", true, err)
	}
	return nil
}

// Get returns a node by ID.
func (s *Service) Get(ctx context.Context, nodeID string) (*NodeDTO, error) {
	n, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, errorsx.New(errorsx.CodeNodeNotReady, "node not found", false)
	}
	return toDTO(n), nil
}

// DefaultBaselineChecker enforces the P0 baseline rules per design §6.6.
type DefaultBaselineChecker struct {
	SELinuxRequired bool
	AllowedKernels  map[string]struct{}
}

// Validate returns nil if the baseline passes, an error otherwise.
func (c *DefaultBaselineChecker) Validate(b models.NodeBaseline) error {
	if c.SELinuxRequired && b.SELinuxStatus != "enforcing" {
		return errors.New("baseline: SELinux must be enforcing")
	}
	if b.KernelVersion == "" {
		return errors.New("baseline: kernel version required")
	}
	if len(c.AllowedKernels) > 0 {
		if _, ok := c.AllowedKernels[b.KernelVersion]; !ok {
			return fmt.Errorf("baseline: kernel %s not in allowlist", b.KernelVersion)
		}
	}
	if b.TPM2TSSVersion == "" {
		return errors.New("baseline: tpm2-tss version required")
	}
	if !b.SwtpmIsolated {
		return errors.New("baseline: swtpm must be isolated")
	}
	return nil
}

// HashNodeID returns a hashed node ID for logging.
func HashNodeID(nodeID string) string {
	h := sha256.Sum256([]byte(nodeID))
	return hex.EncodeToString(h[:])
}

func toDTO(n *models.Node) *NodeDTO {
	return &NodeDTO{
		NodeID:           n.NodeID,
		Role:             n.Role,
		Status:           n.Status,
		ReadyReason:      n.ReadyReason,
		ClusterEpoch:     n.ClusterEpoch,
		AttestationEpoch: n.AttestationEpoch,
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
	}
}
