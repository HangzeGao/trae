package aad

import (
	"bytes"
	"errors"
	"testing"
)

// Fuzz tests for AAD Canonical TLV encoding per design §8.3.

// FuzzEncodeDecode verifies that Encode -> Decode round-trips for valid inputs
// and never panics for arbitrary inputs.
func FuzzEncodeDecode(f *testing.F) {
	// Seed corpus.
	f.Add([]byte{0x01, 0x00, 0x01, 0x41}) // tag 1, len 1, value "A"
	f.Add([]byte{})                        // empty
	f.Add([]byte{0x01, 0x00, 0x00})       // tag 1, len 0
	f.Add([]byte{0xff, 0xff, 0xff, 0xff}) // garbage

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode MUST NOT panic.
		fields, err := Decode(data)
		if err != nil {
			// All errors must be ErrInvalid.
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("non-ErrInvalid error: %v", err)
			}
			return
		}
		// If decoding succeeded, re-encode and verify round-trip.
		reencoded, err := Encode(fields)
		if err != nil {
			t.Fatalf("Encode after Decode failed: %v", err)
		}
		if !bytes.Equal(data, reencoded) {
			t.Fatalf("round-trip mismatch:\n got  %x\n want %x", reencoded, data)
		}
	})
}

// FuzzCallerAADCanonical verifies that CallerAAD.Canonical() never panics and
// produces decodable output for arbitrary string inputs.
func FuzzCallerAADCanonical(f *testing.F) {
	// Seed order MUST match fuzz target signature:
	// (tenantID, keyID, purpose, resourceID string, keyVersion uint32, suiteID uint16)
	f.Add("t", "k", "p", "", uint32(1), uint16(1))
	f.Add("", "", "", "", uint32(0), uint16(0))
	f.Add("tenant-very-long-name-1234567890", "key-abc", "purpose-x", "res-1", uint32(999), uint16(0x0002))

	f.Fuzz(func(t *testing.T, tenantID, keyID, purpose, resourceID string, keyVersion uint32, suiteID uint16) {
		// Cap input sizes to avoid huge allocations.
		if len(tenantID) > 1024 || len(keyID) > 1024 || len(purpose) > 1024 || len(resourceID) > 1024 {
			t.Skip()
		}
		c := CallerAAD{
			TenantID:   tenantID,
			KeyID:      keyID,
			KeyVersion: keyVersion,
			Purpose:    purpose,
			SuiteID:    suiteID,
			ResourceID: resourceID,
		}
		// Canonical MUST NOT panic.
		encoded, err := c.Canonical()
		if err != nil {
			// Errors are acceptable (e.g., field too long).
			return
		}
		// Decoded output must be valid.
		fields, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode of Canonical output failed: %v", err)
		}
		// Verify field count: 5 required + 1 optional if resource_id is non-empty.
		wantCount := 5
		if resourceID != "" {
			wantCount = 6
		}
		if len(fields) != wantCount {
			t.Fatalf("field count = %d, want %d", len(fields), wantCount)
		}
	})
}
