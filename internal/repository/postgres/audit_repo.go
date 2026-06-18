package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
)

// AuditRepository 是审计事件的 PostgreSQL 仓储实现。
//
// 审计事件表为 INSERT only（HA-06）：仅提供插入接口，不提供更新或删除，
// 确保审计记录一旦写入即不可篡改。
type AuditRepository struct{}

// NewAuditRepository 创建一个 AuditRepository 实例。
func NewAuditRepository() *AuditRepository {
	return &AuditRepository{}
}

// marshalDetails 将事件详情 map 序列化为 JSON 字节，nil 返回 nil。
func marshalDetails(details map[string]interface{}) ([]byte, error) {
	if len(details) == 0 {
		return nil, nil
	}
	return json.Marshal(details)
}

// Insert 在事务中插入单条审计事件（INSERT only，HA-06）。
//
// timestamp 若为零值则由数据库默认值（NOW()）填充。
// details 字段序列化为 JSONB 存储。
func (r *AuditRepository) Insert(ctx context.Context, tx *sql.Tx, event *domain.Event) error {
	const q = `INSERT INTO audit_events
		(id, tenant_id, event_type, severity, actor, action, resource_type,
		 resource_id, result, request_id, details, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

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

	_, err = tx.ExecContext(ctx, q,
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

// InsertBatch 在事务中批量插入审计事件（INSERT only，HA-06）。
//
// 使用多值 INSERT（VALUES (...), (...), ...）减少往返次数。
// 若 events 为空则直接返回 nil。
func (r *AuditRepository) InsertBatch(ctx context.Context, tx *sql.Tx, events []*domain.Event) error {
	if len(events) == 0 {
		return nil
	}

	// 构建多值 INSERT 语句：每条事件 12 个占位符
	const cols = 12
	var (
		placeholders strings.Builder
		args         = make([]interface{}, 0, len(events)*cols)
	)
	for i, e := range events {
		if i > 0 {
			placeholders.WriteByte(',')
		}
		base := i * cols
		// ($n, $n+1, ..., $n+11)
		placeholders.WriteByte('(')
		for j := 0; j < cols; j++ {
			if j > 0 {
				placeholders.WriteByte(',')
			}
			fmt.Fprintf(&placeholders, "$%d", base+j+1)
		}
		placeholders.WriteByte(')')

		details, err := marshalDetails(e.Details)
		if err != nil {
			return fmt.Errorf("marshal audit details for event %s: %w", e.ID, err)
		}

		var resourceID interface{}
		if e.ResourceID != "" {
			resourceID = e.ResourceID
		}
		var requestID interface{}
		if e.RequestID != "" {
			requestID = e.RequestID
		}
		var timestamp interface{}
		if !e.Timestamp.IsZero() {
			timestamp = e.Timestamp
		}

		args = append(args,
			e.ID,
			e.TenantID,
			e.EventType,
			string(e.Severity),
			e.Actor,
			e.Action,
			e.ResourceType,
			resourceID,
			e.Result,
			requestID,
			details,
			timestamp,
		)
	}

	q := `INSERT INTO audit_events
		(id, tenant_id, event_type, severity, actor, action, resource_type,
		 resource_id, result, request_id, details, timestamp)
		VALUES ` + placeholders.String()

	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("batch insert audit events: %w", err)
	}
	return nil
}
