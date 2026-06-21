// Shared types matching the Go backend DTOs.

export interface KeyDTO {
  key_id: string;
  tenant_id: string;
  name: string;
  purpose: string;
  policy_id: string;
  suite_id: string;
  current_version: number;
  status: KeyStatus;
  tags?: Record<string, string>;
  created_at: string;
}

export type KeyStatus =
  | "ACTIVE"
  | "DISABLED"
  | "DESTROY_PENDING"
  | "DESTROYED";

export interface NodeDTO {
  node_id: string;
  role: string;
  status: NodeStatus;
  ready_reason: string;
  cluster_epoch: number;
  attestation_epoch: number;
  created_at: string;
  updated_at: string;
  baseline?: NodeBaseline;
}

export type NodeStatus =
  | "REGISTERED"
  | "READY"
  | "DEGRADED"
  | "REVOKED";

export interface NodeBaseline {
  selinux_status: string;
  kernel_version: string;
  virt_platform: string;
  tpm2_tss_version: string;
  swtpm_isolated: boolean;
}

export interface EncryptResponse {
  key_id: string;
  key_version: number;
  suite_id: string;
  ciphertext: string;
}

export interface DecryptResponse {
  key_id: string;
  key_version: number;
  plaintext: string;
}

export interface DataKeyResponse {
  key_id: string;
  key_version: number;
  plaintext_data_key: string;
  wrapped_data_key: string;
  suite_id: string;
  client_zeroize_by: string;
  encryption_context_hash: string;
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    retryable?: boolean;
  };
}

export interface CreateKeyReq {
  tenant_id: string;
  name: string;
  purpose: string;
  policy_id: string;
  suite_id: string;
  tags?: Record<string, string>;
}

export interface EncryptReq {
  tenant_id: string;
  key_id: string;
  plaintext: string;
  aad?: { purpose?: string; resource_id?: string };
  node_id?: string;
}

export interface DecryptReq {
  tenant_id: string;
  ciphertext: string;
  aad?: { purpose?: string; resource_id?: string };
}

export interface DataKeyReq {
  tenant_id: string;
  key_id: string;
  purpose: string;
  ttl_seconds?: number;
  encryption_context?: Record<string, string>;
  caller?: string;
}

export interface RegisterNodeReq {
  node_id: string;
  role: string;
  baseline: NodeBaseline;
}

// Envelope v1 parsed structure (client-side parse for the sandbox).
export interface ParsedEnvelope {
  magic: string;
  version: number;
  flags: number;
  suite_id: string;
  key_id: string;
  key_version: number;
  policy_version: number;
  nonce_len: number;
  tag_len: number;
  ciphertext_len: number;
  aad_hash: string;
  nonce: string;
  ciphertext: string;
  tag: string;
}
