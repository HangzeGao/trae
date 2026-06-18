-- 001_init.sql 初始迁移：创建 key-vault 核心数据模型（第 11.1 节）
-- 包含 keys / key_versions / policies / nodes / audit_events /
--      idempotency_keys / outbox_events 七张表及索引。

-- 启用 pgcrypto 扩展（gen_random_uuid 等函数依赖）
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- keys 表：密钥聚合根
CREATE TABLE keys (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    name VARCHAR(256) NOT NULL,
    algorithm VARCHAR(32) NOT NULL,
    purpose VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    current_version INT NOT NULL DEFAULT 0,
    ready_reason VARCHAR(64) DEFAULT 'static_registration',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    destroyed_at TIMESTAMPTZ
);

-- key_versions 表：密钥版本，包含 wrapped DEK
CREATE TABLE key_versions (
    id VARCHAR(64) PRIMARY KEY,
    key_id VARCHAR(64) NOT NULL REFERENCES keys(id),
    version INT NOT NULL,
    wrapped_dek BYTEA NOT NULL,
    dek_kid VARCHAR(128) NOT NULL,
    kid VARCHAR(256) NOT NULL,  -- 对外暴露的 key_id/version 标识
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128) NOT NULL,
    rotation_reason VARCHAR(64) NOT NULL DEFAULT 'initial',
    UNIQUE(key_id, version)
);

-- policies 表：加密策略
CREATE TABLE policies (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    key_id VARCHAR(64) NOT NULL REFERENCES keys(id),
    min_algorithm VARCHAR(32) NOT NULL,
    require_aad BOOLEAN NOT NULL DEFAULT FALSE,
    max_plaintext_size BIGINT NOT NULL DEFAULT 0,
    allowed_callers TEXT[] DEFAULT '{}',
    cbc_decrypt_only BOOLEAN NOT NULL DEFAULT FALSE,
    ecb_decrypt_only BOOLEAN NOT NULL DEFAULT FALSE,
    downgrade_requires_approval BOOLEAN NOT NULL DEFAULT FALSE,
    approval_id VARCHAR(128),
    policy_signature BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- nodes 表：节点注册信息
CREATE TABLE nodes (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    hostname VARCHAR(256) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'REGISTERED',
    cluster_epoch BIGINT NOT NULL DEFAULT 0,
    attestation_epoch BIGINT NOT NULL DEFAULT 0,
    ready_reason VARCHAR(128),
    service_token_hash VARCHAR(256),
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ
);

-- audit_events 表：审计事件（INSERT only，HA-06）
CREATE TABLE audit_events (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    severity VARCHAR(32) NOT NULL DEFAULT 'info',
    actor VARCHAR(128) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128),
    result VARCHAR(32) NOT NULL,
    request_id VARCHAR(128),
    details JSONB,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- idempotency_keys 表：幂等键
CREATE TABLE idempotency_keys (
    key VARCHAR(256) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    request_hash VARCHAR(256) NOT NULL,
    response_status INT,
    response_body BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- outbox_events 表：发件箱事件
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(128) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

-- 索引
CREATE INDEX idx_keys_tenant_name ON keys(tenant_id, name);
CREATE INDEX idx_keys_tenant_status ON keys(tenant_id, status);
CREATE INDEX idx_key_versions_key_id ON key_versions(key_id);
CREATE INDEX idx_key_versions_dek_kid ON key_versions(dek_kid);
CREATE INDEX idx_policies_tenant_key ON policies(tenant_id, key_id);
CREATE INDEX idx_nodes_tenant_status ON nodes(tenant_id, status);
CREATE INDEX idx_audit_events_tenant_type ON audit_events(tenant_id, event_type, timestamp DESC);
CREATE INDEX idx_audit_events_timestamp ON audit_events(timestamp DESC);
CREATE INDEX idx_idempotency_expires ON idempotency_keys(expires_at);
CREATE INDEX idx_outbox_unprocessed ON outbox_events(processed_at) WHERE processed_at IS NULL;
