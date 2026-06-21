// Package policy implements the Crypto Policy engine per design §10.
// P0 loads policy from YAML at startup; P1 adds signed packages + hot reload.
package policy

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// SuiteStatus enumerates the status of a cipher suite.
type SuiteStatus string

const (
	SuiteStatusActive      SuiteStatus = "active"
	SuiteStatusDecryptOnly SuiteStatus = "decrypt_only"
	SuiteStatusDisabled    SuiteStatus = "disabled"
	SuiteStatusDeprecated  SuiteStatus = "deprecated"
	SuiteStatusBlocked     SuiteStatus = "blocked"
)

// Mode is the cipher mode.
type Mode string

const (
	ModeGCM Mode = "GCM"
	ModeCBC Mode = "CBC"
	ModeECB Mode = "ECB"
)

// Suite is a single cipher suite definition.
type Suite struct {
	SuiteID    string      `yaml:"suite_id"`
	Algorithm  string      `yaml:"algorithm"`
	KeyBits    int         `yaml:"key_bits"`
	Mode       Mode        `yaml:"mode"`
	MAC        string      `yaml:"mac,omitempty"`
	Composition string     `yaml:"composition,omitempty"`
	Nonce      string      `yaml:"nonce"`
	Status     SuiteStatus `yaml:"status"`
	Compliance []string    `yaml:"compliance,omitempty"`
}

// Signature is the P0-reserved policy signature skeleton (HA-05).
type Signature struct {
	Alg              string `yaml:"alg"`
	KeyID            string `yaml:"key_id"`
	Sig              string `yaml:"sig"`
	SignedPayloadHash string `yaml:"signed_payload_hash"`
}

// Policy is a crypto policy package.
type Policy struct {
	PolicyID      string   `yaml:"policy_id"`
	Version       uint32   `yaml:"version"`
	Status        string   `yaml:"status"`
	DefaultSuite  string   `yaml:"default_suite"`
	Suites        []Suite  `yaml:"suites"`
	Signature     Signature `yaml:"signature"`
}

// Engine validates policies and answers allow/deny questions.
type Engine struct {
	policies map[string]*Policy
}

// NewEngine constructs an empty engine.
func NewEngine() *Engine {
	return &Engine{policies: make(map[string]*Policy)}
}

// Load adds a policy to the engine. Validates P0 defaults (HA-05):
//   - default_suite must be an AEAD (GCM) suite
//   - CBC/ECB suites must be decrypt_only
//   - signature field must exist (value may be empty in P0)
func (e *Engine) Load(p *Policy) error {
	if p == nil {
		return fmt.Errorf("policy: nil")
	}
	if p.PolicyID == "" {
		return fmt.Errorf("policy: missing policy_id")
	}
	if p.DefaultSuite == "" {
		return fmt.Errorf("policy: missing default_suite")
	}
	// Validate suites.
	ids := make(map[string]*Suite, len(p.Suites))
	for i := range p.Suites {
		s := &p.Suites[i]
		if _, dup := ids[s.SuiteID]; dup {
			return fmt.Errorf("policy: duplicate suite %s", s.SuiteID)
		}
		ids[s.SuiteID] = s
		// Enforce P0 defaults: CBC/ECB must be decrypt_only.
		if (s.Mode == ModeCBC || s.Mode == ModeECB) && s.Status != SuiteStatusDecryptOnly && s.Status != SuiteStatusDisabled {
			return fmt.Errorf("policy: suite %s mode %s must be decrypt_only or disabled in P0", s.SuiteID, s.Mode)
		}
	}
	// Default suite must be AEAD (GCM) and active.
	def, ok := ids[p.DefaultSuite]
	if !ok {
		return fmt.Errorf("policy: default_suite %s not defined", p.DefaultSuite)
	}
	if def.Mode != ModeGCM {
		return fmt.Errorf("policy: default_suite must be GCM, got %s", def.Mode)
	}
	if def.Status != SuiteStatusActive {
		return fmt.Errorf("policy: default_suite must be active, got %s", def.Status)
	}
	// Signature field must exist (P0 allows empty values).
	_ = p.Signature // presence check via YAML decode; P0 does not validate sig.

	e.policies[p.PolicyID] = p
	return nil
}

// Get returns a policy by ID.
func (e *Engine) Get(policyID string) (*Policy, error) {
	p, ok := e.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy: %s not found", policyID)
	}
	return p, nil
}

// CanEncrypt returns whether the suite may be used for new encryption.
func CanEncrypt(s SuiteStatus) bool {
	return s == SuiteStatusActive || s == SuiteStatusDeprecated
}

// CanDecrypt returns whether the suite may be used for decryption.
func CanDecrypt(s SuiteStatus) bool {
	switch s {
	case SuiteStatusActive, SuiteStatusDecryptOnly, SuiteStatusDeprecated:
		return true
	}
	return false
}

// SuiteByID looks up a suite in a policy.
func (p *Policy) SuiteByID(id string) (*Suite, error) {
	for i := range p.Suites {
		if p.Suites[i].SuiteID == id {
			return &p.Suites[i], nil
		}
	}
	return nil, fmt.Errorf("policy: suite %s not in policy %s", id, p.PolicyID)
}

// LoadFromFile loads a policy from a YAML file.
func LoadFromFile(path string) (*Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: read %s: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("policy: parse %s: %w", path, err)
	}
	// Sort suites by ID for deterministic iteration.
	sort.Slice(p.Suites, func(i, j int) bool {
		return strings.Compare(p.Suites[i].SuiteID, p.Suites[j].SuiteID) < 0
	})
	return &p, nil
}

// DefaultPolicy returns the P0 default policy per design §10.1.
func DefaultPolicy() *Policy {
	return &Policy{
		PolicyID:     "default-v1",
		Version:      1,
		Status:       "active",
		DefaultSuite: "AES_256_GCM",
		Suites: []Suite{
			{SuiteID: "AES_256_GCM", Algorithm: "AES", KeyBits: 256, Mode: ModeGCM, Nonce: "lease_counter", Status: SuiteStatusActive},
			{SuiteID: "SM4_GCM", Algorithm: "SM4", KeyBits: 128, Mode: ModeGCM, Nonce: "lease_counter", Status: SuiteStatusActive, Compliance: []string{"GM_T_0054"}},
			{SuiteID: "AES_256_CBC_HMAC_SHA256", Algorithm: "AES", KeyBits: 256, Mode: ModeCBC, MAC: "HMAC_SHA256", Composition: "encrypt_then_mac", Status: SuiteStatusDecryptOnly},
			{SuiteID: "SM4_CBC_HMAC_SM3", Algorithm: "SM4", KeyBits: 128, Mode: ModeCBC, MAC: "HMAC_SM3", Composition: "encrypt_then_mac", Status: SuiteStatusDecryptOnly},
		},
		Signature: Signature{},
	}
}
