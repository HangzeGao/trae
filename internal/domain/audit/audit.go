// Package audit 定义审计事件领域类型。
package audit

import (
	"context"
	"time"
)

// EventSeverity 事件严重级别。
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityCritical EventSeverity = "critical"
)

// Event 是审计事件。
type Event struct {
	ID           string
	TenantID     string
	EventType    string // key.created, key.rotated, key.destroyed, etc.
	Severity     EventSeverity
	Actor        string // 谁触发的
	Action       string // 具体动作
	ResourceType string // key, node, policy, crk
	ResourceID   string
	Result       string // success, failure, denied
	RequestID    string
	Details      map[string]interface{}
	Timestamp    time.Time
}

// HighRiskEventTypes 高风险事件类型（对应 AuditConfig.HighRiskOps）。
var HighRiskEventTypes = map[string]bool{
	"crk.create":       true,
	"crk.rotate":       true,
	"node.register":    true,
	"node.revoke":      true,
	"key.destroy":      true,
	"policy.downgrade": true,
	"key.rotate":       true,
}

// IsHighRisk 判断事件是否为高风险。
func (e *Event) IsHighRisk() bool {
	return HighRiskEventTypes[e.EventType]
}

// Sink 是审计事件投递接口。
type Sink interface {
	Emit(ctx context.Context, event Event) error
}
