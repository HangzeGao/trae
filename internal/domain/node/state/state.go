// Package nodestate implements the Node state machine per design §14.3.
package nodestate

import "fmt"

// Status enumerates Node states.
type Status string

const (
	StatusRegistered Status = "REGISTERED"
	StatusReady      Status = "READY"
	StatusDegraded   Status = "DEGRADED"
	StatusRevoked    Status = "REVOKED"
)

// Event is a trigger for a Node state transition.
type Event string

const (
	EvAuthOrAttestationPass Event = "auth_or_attestation_pass"
	EvHealthWarning         Event = "health_warning"
	EvRecovered             Event = "recovered"
	EvRevoke                Event = "revoke"
)

// Transition returns the new Node status for a (current, event) pair.
func Transition(current Status, event Event) (Status, error) {
	switch current {
	case StatusRegistered:
		switch event {
		case EvAuthOrAttestationPass:
			return StatusReady, nil
		}
	case StatusReady:
		switch event {
		case EvHealthWarning:
			return StatusDegraded, nil
		case EvRevoke:
			return StatusRevoked, nil
		}
	case StatusDegraded:
		switch event {
		case EvRecovered:
			return StatusReady, nil
		case EvRevoke:
			return StatusRevoked, nil
		}
	case StatusRevoked:
		// Terminal
	}
	return current, fmt.Errorf("nodestate: illegal transition %s -> %s", current, event)
}

// CanReceiveDEKLease returns whether a node in the given status may
// receive DEK leases (data plane). Per design §9.1, only READY nodes.
func CanReceiveDEKLease(s Status) bool {
	return s == StatusReady
}

// CanReceiveCRKEnvelope returns whether a node may receive CRK envelopes
// (management/key plane). Per design §9.1, only READY nodes.
func CanReceiveCRKEnvelope(s Status) bool {
	return s == StatusReady
}
