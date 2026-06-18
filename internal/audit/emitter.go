// Package audit 提供审计事件发射器，协调 WAL、DB、脱敏等组件。
//
// Emitter 是审计事件发射器，根据事件是否为高风险采用不同策略：
//   - 高风险操作（event.IsHighRisk()）：先写 WAL（fail-closed），再写 DB（fail-closed）
//   - 普通操作：直接写 DB（fail-open，记录错误但继续）
//
// 所有审计事件在写入前都会经过脱敏处理，确保敏感字段不会泄露。
package audit

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/audit/redaction"
	"github.com/HangzeGao/trae/key-vault/internal/audit/sink"
	"github.com/HangzeGao/trae/key-vault/internal/audit/wal"
	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// auditInsertSQL 是审计事件插入语句（INSERT only，HA-06）。
const auditInsertSQL = `INSERT INTO audit_events
	(id, tenant_id, event_type, severity, actor, action, resource_type,
	 resource_id, result, request_id, details, timestamp)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

// Emitter 是审计事件发射器，协调 WAL、DB、脱敏。
//
// 高风险操作采用 fail-closed 策略：WAL 或 DB 写入失败则返回错误，
// 调用方应回滚业务事务。
// 普通操作采用 fail-open 策略：DB 写入失败仅记录日志，不影响业务流程。
type Emitter struct {
	dbSink   domain.Sink
	walSink  domain.Sink // 可选，仅高风险操作使用
	redactor *redaction.Redactor
	log      *observability.Logger
	db       *sql.DB // 用于 EmitWithTx 直接操作
}

// NewEmitter 创建一个 Emitter 实例。
//
// db 用于数据库写入，wal_ 用于高风险操作的本地 WAL（可为 nil 表示不启用 WAL），
// log 用于记录 fail-open 场景下的错误日志。
func NewEmitter(db *sql.DB, wal_ *wal.WAL, log *observability.Logger) *Emitter {
	dbSink := sink.NewDBSink(db, log)

	var walSink domain.Sink
	if wal_ != nil {
		walSink = sink.NewWALSink(wal_, log)
	}

	return &Emitter{
		dbSink:   dbSink,
		walSink:  walSink,
		redactor: redaction.NewRedactor(),
		log:      log,
		db:       db,
	}
}

// Emit 发射审计事件。
//
// 高风险操作（event.IsHighRisk()）：先写 WAL（fail-closed），再写 DB（fail-closed）。
// 普通操作：直接写 DB（fail-open，记录错误但继续）。
//
// 所有事件在写入前都会经过脱敏处理。
func (e *Emitter) Emit(ctx context.Context, event domain.Event) error {
	// 脱敏处理
	e.redactor.Redact(&event)

	highRisk := event.IsHighRisk()

	// 高风险操作：先写 WAL（fail-closed）
	if highRisk && e.walSink != nil {
		if err := e.walSink.Emit(ctx, event); err != nil {
			return fmt.Errorf("wal emit (fail-closed): %w", err)
		}
	}

	// 写入 DB
	if err := e.dbSink.Emit(ctx, event); err != nil {
		if highRisk {
			// 高风险操作 fail-closed：返回错误，调用方应回滚业务事务
			return fmt.Errorf("db emit (fail-closed): %w", err)
		}
		// 普通操作 fail-open：记录错误但继续
		e.log.Error("audit db emit failed (fail-open)",
			"error", err,
			"event_type", event.EventType,
			"event_id", event.ID,
			"tenant_id", event.TenantID,
		)
		return nil
	}

	return nil
}

// EmitWithTx 在事务内发射审计事件（用于业务事务内审计）。
//
// 高风险操作：先写 WAL（fail-closed），再在事务内写 DB（fail-closed）。
// 普通操作：直接在事务内写 DB（fail-open，记录错误但继续）。
//
// 使用提供的 tx 执行 DB 写入，确保审计事件与业务操作在同一事务中提交或回滚。
// 所有事件在写入前都会经过脱敏处理。
func (e *Emitter) EmitWithTx(ctx context.Context, tx *sql.Tx, event domain.Event) error {
	// 脱敏处理
	e.redactor.Redact(&event)

	highRisk := event.IsHighRisk()

	// 高风险操作：先写 WAL（fail-closed）
	if highRisk && e.walSink != nil {
		if err := e.walSink.Emit(ctx, event); err != nil {
			return fmt.Errorf("wal emit (fail-closed): %w", err)
		}
	}

	// 在事务内写入 DB
	if err := e.insertWithTx(ctx, tx, &event); err != nil {
		if highRisk {
			// 高风险操作 fail-closed：返回错误，调用方应回滚业务事务
			return fmt.Errorf("db emit with tx (fail-closed): %w", err)
		}
		// 普通操作 fail-open：记录错误但继续
		e.log.Error("audit db emit with tx failed (fail-open)",
			"error", err,
			"event_type", event.EventType,
			"event_id", event.ID,
			"tenant_id", event.TenantID,
		)
		return nil
	}

	return nil
}

// insertWithTx 在事务内执行审计事件 INSERT（INSERT only，HA-06）。
func (e *Emitter) insertWithTx(ctx context.Context, tx *sql.Tx, event *domain.Event) error {
	details, err := marshalDetails(event.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	var resourceID interface{}
	if event.ResourceID != "" {
		resourceID = event.ResourceID
	}
	var requestID interface{}
	if event.RequestID != "" {
		requestID = event.RequestID
	}
	var timestamp interface{}
	if !event.Timestamp.IsZero() {
		timestamp = event.Timestamp
	}

	_, err = tx.ExecContext(ctx, auditInsertSQL,
		event.ID,
		event.TenantID,
		event.EventType,
		string(event.Severity),
		event.Actor,
		event.Action,
		event.ResourceType,
		resourceID,
		event.Result,
		requestID,
		details,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// marshalDetails 将事件详情 map 序列化为 JSON 字节，nil 或空则返回 nil。
func marshalDetails(details map[string]interface{}) ([]byte, error) {
	if len(details) == 0 {
		return nil, nil
	}
	return json.Marshal(details)
}

// generateEventID 生成 UUID v4 格式的事件 ID。
//
// 使用 crypto/rand 生成随机字节，设置版本（4）和变体（10xx）位。
func generateEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// 设置版本位 (version 4)
	b[6] = (b[6] & 0x0f) | 0x40
	// 设置变体位 (variant 10xx)
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// severityForEventType 根据事件类型推断严重级别。
//
// 高风险事件类型返回 SeverityCritical，其他返回 SeverityInfo。
func severityForEventType(eventType string) domain.EventSeverity {
	if domain.HighRiskEventTypes[eventType] {
		return domain.SeverityCritical
	}
	return domain.SeverityInfo
}

// BuildEvent 构建审计事件的辅助函数。
//
// 自动生成事件 ID（UUID v4）、时间戳（当前时间）和严重级别（基于事件类型）。
// details 可为 nil。
func BuildEvent(tenantID, eventType, actor, action, resourceType, resourceID, result, requestID string, details map[string]interface{}) domain.Event {
	return domain.Event{
		ID:           generateEventID(),
		TenantID:     tenantID,
		EventType:    eventType,
		Severity:     severityForEventType(eventType),
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		RequestID:    requestID,
		Details:      details,
		Timestamp:    time.Now(),
	}
}
