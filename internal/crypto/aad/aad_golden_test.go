package aad

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// Golden Test Vectors for AAD Canonical TLV encoding per design §8.3.
// These vectors are immutable and serve as cross-implementation reference.
// Any change to the canonical encoding MUST be a version bump (Envelope v2).

// TestGoldenCanonicalCallerAAD verifies the deterministic TLV encoding of
// CallerAAD with the 5 required fields and no optional resource_id.
func TestGoldenCanonicalCallerAAD(t *testing.T) {
	c := CallerAAD{
		TenantID:   "tenant-001",
		KeyID:      "key-abc",
		KeyVersion: 1,
		Purpose:    "record-encryption",
		SuiteID:    0x0001, // AES_256_GCM
	}
	got, err := c.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}

	// Build expected encoding manually.
	// Field order: 0x01 tenant_id, 0x02 key_id, 0x03 key_version (4 bytes BE),
	//              0x04 purpose, 0x05 suite_id (2 bytes BE).
	var want bytes.Buffer
	writeField := func(tag uint8, val []byte) {
		want.WriteByte(tag)
		var l [2]byte
		binary.BigEndian.PutUint16(l[:], uint16(len(val)))
		want.Write(l[:])
		want.Write(val)
	}
	writeField(0x01, []byte("tenant-001"))
	writeField(0x02, []byte("key-abc"))
	kv := make([]byte, 4)
	binary.BigEndian.PutUint32(kv, 1)
	writeField(0x03, kv)
	writeField(0x04, []byte("record-encryption"))
	suite := make([]byte, 2)
	binary.BigEndian.PutUint16(suite, 0x0001)
	writeField(0x05, suite)

	if !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("canonical mismatch:\n got  %x\n want %x", got, want.Bytes())
	}
}

// TestGoldenCanonicalCallerAADWithResource verifies encoding with optional
// resource_id (field 0x06) appended after the 5 required fields.
func TestGoldenCanonicalCallerAADWithResource(t *testing.T) {
	c := CallerAAD{
		TenantID:   "t1",
		KeyID:      "k1",
		KeyVersion: 7,
		Purpose:    "p",
		SuiteID:    0x0002, // SM4_GCM
		ResourceID: "res-xyz",
	}
	got, err := c.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	// Decode and verify field count + order.
	fields, err := Decode(got)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(fields) != 6 {
		t.Fatalf("expected 6 fields, got %d", len(fields))
	}
	expectedTags := []uint8{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	for i, f := range fields {
		if f.ID != expectedTags[i] {
			t.Fatalf("field %d: tag 0x%02x, want 0x%02x", i, f.ID, expectedTags[i])
		}
	}
	if string(fields[5].Value) != "res-xyz" {
		t.Fatalf("resource_id = %q, want %q", fields[5].Value, "res-xyz")
	}
}

// TestGoldenCanonicalCRKAAD verifies the 7-field CRK AAD encoding.
func TestGoldenCanonicalCRKAAD(t *testing.T) {
	baseline := []byte{0xaa, 0xbb, 0xcc, 0xdd}
	policy := []byte{0x11, 0x22, 0x33, 0x44}
	c := CRKAAD{
		ClusterID:      "cluster-prod",
		NodeID:         "node-1",
		PlaneRole:      "key-plane",
		CRKVersion:     1,
		NRWKName:       "nrwk-primary",
		BaselineDigest: baseline,
		PolicyDigest:   policy,
	}
	got, err := c.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	fields, err := Decode(got)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(fields) != 7 {
		t.Fatalf("expected 7 fields, got %d", len(fields))
	}
	expectedTags := []uint8{0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D}
	for i, f := range fields {
		if f.ID != expectedTags[i] {
			t.Fatalf("field %d: tag 0x%02x, want 0x%02x", i, f.ID, expectedTags[i])
		}
	}
}

// TestGoldenCanonicalRoundTrip verifies Encode -> Decode round-trip preserves data.
func TestGoldenCanonicalRoundTrip(t *testing.T) {
	fields := []Field{
		{ID: 0x01, Value: []byte("alpha")},
		{ID: 0x02, Value: []byte("beta")},
		{ID: 0x03, Value: []byte("gamma")},
	}
	enc, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(dec) != len(fields) {
		t.Fatalf("len = %d, want %d", len(dec), len(fields))
	}
	for i, f := range fields {
		if dec[i].ID != f.ID {
			t.Fatalf("field %d: ID mismatch", i)
		}
		if !bytes.Equal(dec[i].Value, f.Value) {
			t.Fatalf("field %d: value mismatch", i)
		}
	}
}

// TestGoldenCanonicalRejectsOutOfOrder verifies that out-of-order fields are rejected.
func TestGoldenCanonicalRejectsOutOfOrder(t *testing.T) {
	fields := []Field{
		{ID: 0x02, Value: []byte("b")},
		{ID: 0x01, Value: []byte("a")}, // out of order
	}
	if _, err := Encode(fields); err == nil {
		t.Fatal("expected error for out-of-order fields")
	}
}

// TestGoldenCanonicalRejectsRepeated verifies that repeated fields are rejected.
func TestGoldenCanonicalRejectsRepeated(t *testing.T) {
	fields := []Field{
		{ID: 0x01, Value: []byte("a")},
		{ID: 0x01, Value: []byte("b")}, // repeated
	}
	if _, err := Encode(fields); err == nil {
		t.Fatal("expected error for repeated fields")
	}
}

// TestGoldenCanonicalRejectsTrailingBytes verifies that trailing bytes are rejected.
func TestGoldenCanonicalRejectsTrailingBytes(t *testing.T) {
	enc, err := Encode([]Field{{ID: 0x01, Value: []byte("a")}})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Append a trailing byte.
	enc = append(enc, 0xff)
	if _, err := Decode(enc); err == nil {
		t.Fatal("expected error for trailing bytes")
	}
}

// TestGoldenCanonicalEmptyValue verifies that empty values are allowed (len=0).
func TestGoldenCanonicalEmptyValue(t *testing.T) {
	fields := []Field{
		{ID: 0x01, Value: []byte{}},
		{ID: 0x02, Value: nil}, // nil == empty
	}
	enc, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Each field: 1 tag + 2 len + 0 value = 3 bytes.
	if len(enc) != 6 {
		t.Fatalf("len = %d, want 6", len(enc))
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(dec) != 2 {
		t.Fatalf("decoded len = %d, want 2", len(dec))
	}
	if len(dec[0].Value) != 0 || len(dec[1].Value) != 0 {
		t.Fatal("expected empty values")
	}
}

// TestGoldenCanonicalHexVector provides a stable hex vector for cross-language verification.
func TestGoldenCanonicalHexVector(t *testing.T) {
	c := CallerAAD{
		TenantID:   "T",
		KeyID:      "K",
		KeyVersion: 1,
		Purpose:    "P",
		SuiteID:    0x0001,
	}
	got, err := c.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	// Manually compute the expected hex:
	// 0x01 00 01 54                                            (tenant "T")
	// 0x02 00 01 4b                                            (key "K")
	// 0x03 00 04 00 00 00 01                                   (key_version=1)
	// 0x04 00 01 50                                            (purpose "P")
	// 0x05 00 02 00 01                                         (suite=0x0001)
	wantHex := "010001540200014b03000400000001040001500500020001"
	if hex.EncodeToString(got) != wantHex {
		t.Fatalf("hex mismatch:\n got  %x\n want %s", got, wantHex)
	}
}
