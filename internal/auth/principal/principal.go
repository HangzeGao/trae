// Package principal defines the authenticated caller identity.
package principal

import "strings"

// Plane enumerates the API planes a principal may access.
type Plane string

const (
	PlaneManagement Plane = "management"
	PlaneData       Plane = "data"
)

// Role enumerates service roles.
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleKeyPlane   Role = "key"
	RoleDataPlane  Role = "data"
	RoleNode       Role = "node"
)

// Principal is the authenticated caller. Constructed by auth middleware.
type Principal struct {
	ID         string   // stable principal ID (sub or service ID)
	TenantID   string   // tenant scope
	Scopes     []string // e.g. "keys:manage", "crypto:encrypt", "datakey:generate"
	Roles      []string
	Plane      Plane    // which plane this principal may access
	NodeID     string   // if role=node
	AuthMethod string   // "jwt" | "hmac" | "static_token"
}

// HasScope returns whether the principal holds a given scope.
func (p *Principal) HasScope(s string) bool {
	for _, sc := range p.Scopes {
		if sc == s {
			return true
		}
	}
	return false
}

// HasAnyScope returns whether the principal holds any of the given scopes.
func (p *Principal) HasAnyScope(scopes ...string) bool {
	for _, want := range scopes {
		if p.HasScope(want) {
			return true
		}
	}
	return false
}

// HasRole returns whether the principal has a given role.
func (p *Principal) HasRole(r string) bool {
	for _, x := range p.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// CanAccessPlane returns whether the principal may access the given plane.
// Per design §4.5, data plane principals cannot call management APIs.
func (p *Principal) CanAccessPlane(plane Plane) bool {
	if p.Plane == plane {
		return true
	}
	// Admin plane can access both.
	if p.Plane == PlaneManagement && plane == PlaneData {
		return true
	}
	return false
}

// ScopesString returns scopes as a space-joined string.
func (p *Principal) ScopesString() string {
	return strings.Join(p.Scopes, " ")
}
