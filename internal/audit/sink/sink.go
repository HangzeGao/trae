// Package sink 实现审计事件投递 Sink，支持多种投递策略。
//
// 提供以下 Sink 实现：
//   - DBSink: 将审计事件写入数据库 audit_events 表（INSERT only，HA-06）
//   - WALSink: 将审计事件写入本地 WAL（高风险操作用）
//   - CompositeSink: 组合多个 sink，按顺序投递
//   - FailClosedSink: 高风险操作使用，任一 sink 失败则返回错误
//   - FailOpenSink: 普通操作使用，记录错误但继续
package sink

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HangzeGao/trae/key-vault/internal/audit/wal"
	domain "github.com/HangzeGao/trae/key-vault/internal/domain/audit"
	"github.com/HangzeGao/trae/key-vault/internal/observability"
)

// auditInsertSQL 是审计事件插入语句（INSERT only，HA-06）。
const auditInsertSQL = `INSERT INTO audit_events
	(id, tenant_id, event_type, severity, actor, action, resource_type,
	 resource_id, result, request_id, details, timestamp)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

// marshalDetails 将事件详情 map 序列化为 JSON 字节，nil 或空则返回 nil。
func marshalDetails(details map[string]interface{}) ([]byte, error) {
	if len(details) == 0 {
		return nil, nil
	}
	return json.Marshal(details)
}

// nullableString 将空字符串转为 nil（对应 SQL NULL），非空则原样返回。
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullableTime 将零值时间转为 nil（对应 SQL NULL，由数据库默认值填充）。
func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// DBSink 将审计事件写入数据库 audit_events 表。
//
// 仅执行 INSERT 操作（HA-06），不提供更新或删除，
// 确保审计记录一旦写入即不可篡改。
type DBSink struct {
	db  *sql.DB
	log *observability.Logger
}

// NewDBSink 创建一个 DBSink 实例。
func NewDBSink(db *sql.DB, log *observability.Logger) *DBSink {
	return &DBSink{db: db, log: log}
}

// Emit 将事件写入数据库（INSERT only，HA-06）。
//
// 在独立事务中执行 INSERT，确保审计记录原子写入。
// 空字符串字段转为 SQL NULL，零值时间戳由数据库默认值（NOW()）填充。
func (s *DBSink) Emit(ctx context.Context, event domain.Event) error {
	details, err := marshalDetails(event.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	_, err = s.db.ExecContext(ctx, auditInsertSQL,
		event.ID,
		event.TenantID,
		event.EventType,
		string(event.Severity),
		event.Actor,
		event.Action,
		event.ResourceType,
		nullableString(event.ResourceID),
		event.Result,
		nullableString(event.RequestID),
		details,
		nullableTime(event.Timestamp),
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// WALSink 将审计事件写入本地 WAL（高风险操作用）。
//
// 用于高风险操作的 fail-closed 审计（HA-07），
// 确保审计事件在业务事务提交前持久化到本地磁盘。
type WALSink struct {
	wal *wal.WAL
	log *observability.Logger
}

// NewWALSink 创建一个 WALSink 实例。
func NewWALSink(w *wal.WAL, log *observability.Logger) *WALSink {
	return &WALSink{wal: w, log: log}
}

// Emit 将事件写入本地 WAL。
//
// 写入失败时返回错误，调用方应 fail-closed（回滚业务事务）。
func (s *WALSink) Emit(ctx context.Context, event domain.Event) error {
	if _, err := s.wal.Append(ctx, event); err != nil {
		return fmt.Errorf("append to wal: %w", err)
	}
	return nil
}

// CompositeSink 组合多个 sink，按顺序投递。
//
// 任一 sink 失败则立即返回错误，后续 sink 不再投递。
type CompositeSink struct {
	sinks []domain.Sink
}

// NewCompositeSink 创建一个 CompositeSink 实例。
func NewCompositeSink(sinks ...domain.Sink) *CompositeSink {
	return &CompositeSink{sinks: sinks}
}

// Emit 按顺序投递到所有 sink，任一失败则返回错误。
func (s *CompositeSink) Emit(ctx context.Context, event domain.Event) error {
	for _, sn := range s.sinks {
		if err := sn.Emit(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// FailClosedSink 高风险操作使用：任一 sink 失败则返回错误（fail-closed）。
//
// 包装内部 sink，失败时直接返回错误，调用方应回滚业务事务。
type FailClosedSink struct {
	inner domain.Sink
}

// NewFailClosedSink 创建一个 FailClosedSink 实例。
func NewFailClosedSink(inner domain.Sink) *FailClosedSink {
	return &FailClosedSink{inner: inner}
}

// Emit 投递事件，失败时返回错误（fail-closed）。
//
// 高风险操作使用此 Sink，确保审计失败时业务操作也失败。
func (s *FailClosedSink) Emit(ctx context.Context, event domain.Event) error {
	if err := s.inner.Emit(ctx, event); err != nil {
		return err
	}
	return nil
}

// FailOpenSink 普通操作使用：记录错误但继续（fail-open）。
//
// 包装内部 sink，失败时记录日志但返回 nil，
// 确保审计失败不影响普通业务操作。
type FailOpenSink struct {
	inner domain.Sink
	log   *observability.Logger
}

// NewFailOpenSink 创建一个 FailOpenSink 实例。
func NewFailOpenSink(inner domain.Sink, log *observability.Logger) *FailOpenSink {
	return &FailOpenSink{inner: inner, log: log}
}

// Emit 记录错误但返回 nil（fail-open）。
//
// 普通操作使用此 Sink，审计失败时仅记录日志，不影响业务流程。
func (s *FailOpenSink) Emit(ctx context.Context, event domain.Event) error {
	if err := s.inner.Emit(ctx, event); err != nil {
		s.log.Error("audit sink failed (fail-open)",
			"error", err,
			"event_type", event.EventType,
			"event_id", event.ID,
			"tenant_id", event.TenantID,
		)
		return nil
	}
	return nil
}
