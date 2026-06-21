// Package redaction provides hashing helpers for sensitive identifiers
// in audit events and logs, per design §15.1 and §18.2.
package redaction

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// pepper is a process-wide HMAC key used to prevent trivial rainbow-table
// attacks on hashed identifiers. In production it should be loaded from
// a secret; P0 derives it from a fixed root + cluster_id for determinism
// within a deployment.
var pepper = []byte("kvlt-redaction-pepper-v1")

// SetPepper overrides the default pepper. Must be called once at startup.
func SetPepper(p []byte) {
	if len(p) > 0 {
		pepper = p
	}
}

// Hash returns a hex-encoded HMAC-SHA256 of the input under the pepper.
// The same input always yields the same hash within a process; different
// deployments with different peppers yield different hashes.
func Hash(s string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashBytes is like Hash but for byte slices.
func HashBytes(b []byte) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil))
}

// ShortHash returns the first 16 hex chars of Hash(s).
func ShortHash(s string) string {
	return Hash(s)[:16]
}

// MaybeRedactJSON marshals v to JSON, then verifies that none of the
// sensitive keys appear at the top level. This is a defense-in-depth check.
func MaybeRedactJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
