# 基于 TPM 2.0 的软硬结合加解密系统 Spec

## 为什么需要本系统
当前 `internal/crypto` 和 `internal/model` 已实现 AES-128/192/256 + SM4 的 GCM/CBC/CTR/CFB/OFB 多种分组模式加解密能力，但数据密钥以明文形式在内存中流转，缺乏硬件级密钥保护。需要引入 TPM 2.0 作为硬件信任根，实现根密钥（SRK）永不离开芯片、数据密钥由根密钥加密保护的两层密钥体系，并支持密钥服务隔离部署和多节点集群部署。

## 整体架构

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          业务节点 (Business Node)                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │                  EncryptionClient (internal/client)                  │ │
│  │  ┌──────────────────────┐     ┌─────────────────────────────────┐   │ │
│  │  │  本地 AES/SM4 加密    │◀───▶│  与 KeyServer 通信(解密数据密钥)  │   │ │
│  │  │  (复用 internal/crypto)│     │  (多地址故障转移 / TLS)           │   │ │
│  │  └──────────────────────┘     └─────────────────────────────────┘   │ │
│  └──────────────────────────────────┬──────────────────────────────────┘ │
└─────────────────────────────────────┼────────────────────────────────────┘
                                      │ TLS / HTTPS
                                      ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    密钥服务节点 A (KeyServer Node A)                        │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │              HTTP API Server (internal/keyserver)                      │ │
│  │  POST /api/v1/keys/generate   ── 生成并加密数据密钥                     │ │
│  │  POST /api/v1/keys/decrypt    ── TPM 解密数据密钥                        │ │
│  │  POST /api/v1/keys/rotate     ── 轮换数据密钥                             │ │
│  │  GET  /api/v1/algorithms       ── 查询支持的算法                          │ │
│  └────────────────────────┬──────────────────────────────────────────────┘ │
│                           │                                                 │
│  ┌────────────────────────▼──────────────────────────────────────────────┐ │
│  │              internal/tpm 模块                                          │ │
│  │  TPMContext  ── 管理 TPM 连接 / ESAPI 上下文                           │ │
│  │  RootKey     ── SRK 创建(TPM2_CreatePrimary)与加载                       │ │
│  │  DataKey     ── 用 SRK 公钥加密 / 用 TPM2_RSA_Decrypt 解密              │ │
│  └────────────────────────┬──────────────────────────────────────────────┘ │
│                           │                                                 │
│  ┌────────────────────────▼──────────────┐                                  │
│  │         TPM 2.0 硬件芯片               │                                  │
│  │  ┌─────────────────────────────────┐  │                                  │
│  │  │     SRK (Storage Root Key)      │  │                                  │
│  │  │  私钥部分永不离开 TPM 芯片        │  │                                  │
│  │  │  公钥可导出用于离线加密数据密钥    │  │                                  │
│  │  └─────────────────────────────────┘  │                                  │
│  └───────────────────────────────────────┘                                  │
└──────────────────────────────────────────────────────────────────────────┘

       ▲ 可部署多个 KeyServer 节点，共享同一 SRK 公钥实现跨节点解密
       │ 业务节点可配置多个 KeyServer 地址实现故障转移
```

## 两层密钥体系设计

### 第一层：根密钥（SRK - Storage Root Key）
- **位置**：在 TPM 2.0 芯片内，由 `TPM2_CreatePrimary` 创建于 Owner hierarchy
- **类型**：RSA 2048 位受限解密密钥（restricted decryption key）
- **特性**：私钥永不离开 TPM，公钥（public portion）可导出
- **生命周期**：服务首次启动时创建，后续启动直接加载；意外丢失时需重新初始化
- **用途**：
  1. 加密保护第二层的「数据密钥」
  2. 使用 `TPM2_RSA_Decrypt` 在 TPM 内解密数据密钥密文

### 第二层：数据密钥（Data Key）
- **位置**：明文形式仅存在于内存中（使用后清零），密文形式持久化存储或通过网络传输
- **类型**：对称密钥，长度取决于算法（AES-128: 16字节 / AES-192: 24字节 / AES-256: 32字节 / SM4: 16字节）
- **生成**：通过 `crypto/rand` 生成密码学安全的随机密钥
- **加密保护**：使用 SRK 的 RSA 公钥以 OAEP 模式加密
- **用途**：直接用于业务数据的 AES/SM4 加解密

### 密钥流转核心流程

**流程 1：生成数据密钥**
```
1. KeyServer 内部: crypto/rand.Read(plaintext_data_key)  ← 生成随机密钥
2. KeyServer 内部: rsa.EncryptOAEP(srk_public_key, plaintext_data_key)  ← 用 SRK 公钥加密
3. KeyServer → Client: 返回 (encrypted_data_key, plaintext_data_key)
4. Client: 缓存 plaintext_data_key 于内存，存储 encrypted_data_key 到持久层
```

**流程 2：加密业务数据**
```
1. Client: 检查内存中是否已有 plaintext_data_key
   ├─ 已有 → 直接用于加解密
   └─ 没有 → 发送 POST /api/v1/keys/decrypt(encrypted_data_key) 到 KeyServer
2. KeyServer → TPM: TPM2_RSA_Decrypt(SRK_handle, encrypted_data_key)  ← 在 TPM 内解密
3. KeyServer → Client: 返回 plaintext_data_key
4. Client: internal/crypto.Cipher.Encrypt(plaintext)  ← 本地加密
5. Client: 存储 CiphertextFormat 序列化结果
```

**流程 3：解密业务数据**
```
1. Client: 反序列化 CiphertextFormat
2. Client: 获取 plaintext_data_key（同流程 2 的密钥解密步骤）
3. Client: internal/crypto.Cipher.Decrypt(ciphertext_format)  ← 本地解密
4. Client: 零化内存中的 plaintext_data_key
```

**流程 4：密钥轮换**
```
1. Client: 发送 POST /api/v1/keys/rotate(old_encrypted_data_key) 到 KeyServer
2. KeyServer:
   ├─ 用 SRK 解密 old_encrypted_data_key 验证密钥有效（可选）
   ├─ crypto/rand.Read(new_plaintext_data_key)  ← 生成新密钥
   └─ rsa.EncryptOAEP(SRK_public_key, new_plaintext_data_key)  ← 用 SRK 加密
3. KeyServer → Client: 返回 (new_encrypted_data_key, new_plaintext_data_key)
4. Client: 使用旧密钥解密存量数据，再用新密钥重新加密
5. Client: 废弃旧 encrypted_data_key
```

## 模块划分与变更清单

| 模块 | 路径 | 状态 | 说明 |
|------|------|------|------|
| TPM 驱动层 | `internal/tpm/` | 新增 | 封装 TPM 2.0 操作：上下文管理、SRK 创建、密钥加密解密 |
| 密钥服务层 | `internal/keyserver/` | 新增 | HTTP API Server + 业务逻辑编排 |
| 加解密客户端 | `internal/client/` | 新增 | 业务节点使用的客户端，封装与 KeyServer 的通信和本地加解密 |
| 配置管理 | `internal/config/` | 新增 | 服务与客户端的配置加载 |
| 对称密码层 | `internal/crypto/` | 已有 | [cipher.go](file:///workspace/internal/crypto/cipher.go) + [sm4.go](file:///workspace/internal/crypto/sm4.go)，无需修改 |
| 模型层 | `internal/model/` | 已有 | 已有 [api.go](file:///workspace/internal/model/api.go) + [algorithm.go](file:///workspace/internal/model/algorithm.go) + [ciphertext.go](file:///workspace/internal/model/ciphertext.go) + [errors.go](file:///workspace/internal/model/errors.go)，需补充 DecryptKey 请求/响应 |
| 启动入口 | `cmd/keyserver/`, `cmd/client/` | 新增 | 服务端与客户端示例入口 |

## 外部依赖

| 依赖 | 用途 |
|------|------|
| `github.com/google/go-tpm/tpm2` | TPM 2.0 设备操作（打开设备、创建主密钥、RSA 解密） |
| `crypto/rsa`, `crypto/rand` | 标准库 — 数据密钥加密（OAEP）、随机数生成 |
| `net/http` | 标准库 — HTTP 服务端与客户端 |
| `encoding/json` | 标准库 — 请求响应序列化 |

## 部署模式

### 模式一：隔离部署（推荐生产环境）
- **架构**：一台或多台专用 KeyServer 物理机（带 TPM 2.0 芯片），多台业务节点
- **数据流**：业务节点通过 HTTPS 请求 KeyServer 解密数据密钥，本地执行加解密
- **优点**：TPM 仅暴露于专用节点，最小化攻击面；密钥服务独立扩容

### 模式二：单节点共部署
- **架构**：业务服务与 KeyServer 运行在同一台物理机
- **优点**：延迟最低；适合单机部署场景
- **注意**：TPM 设备需要授予业务进程访问权限

### 模式三：集群部署（多 KeyServer 实例）
- **架构**：部署 N 台 KeyServer，每台有独立 TPM 和独立 SRK
- **跨节点兼容性**：每台 KeyServer 的 SRK 公钥相同（从同一模板创建），加密数据密钥可在任一实例解密
- **客户端故障转移**：业务节点配置多个 KeyServer 地址，按顺序尝试连接
- **负载均衡**：可在 KeyServer 前端放置 L4/L7 负载均衡器

## API 协议设计

### POST /api/v1/keys/generate — 生成数据密钥

请求体 [GenerateKeyRequest](file:///workspace/internal/model/api.go#L4-L6)：
```json
{ "algorithm": "AES-256" }
```

响应体 [GenerateKeyResponse](file:///workspace/internal/model/api.go#L9-L13)：
```json
{
  "encrypted_data_key": "base64(RSA-OAEP(SRK_pub, AES_key))",
  "plaintext_data_key": "base64(AES_key)",
  "algorithm": "AES-256"
}
```

### POST /api/v1/keys/decrypt — 解密数据密钥（新增）

请求体 `DecryptKeyRequest`：
```json
{
  "encrypted_data_key": "base64(RSA-OAEP(SRK_pub, AES_key))"
}
```

响应体 `DecryptKeyResponse`：
```json
{
  "plaintext_data_key": "base64(AES_key)"
}
```

### POST /api/v1/keys/rotate — 轮换数据密钥

请求体 [RotateKeyRequest](file:///workspace/internal/model/api.go#L42-L45)：
```json
{
  "old_encrypted_data_key": "base64(...)",
  "algorithm": "AES-256"
}
```

响应体 [RotateKeyResponse](file:///workspace/internal/model/api.go#L48-L52)：
```json
{
  "new_encrypted_data_key": "base64(...)",
  "new_plaintext_data_key": "base64(...)",
  "algorithm": "AES-256"
}
```

### GET /api/v1/algorithms — 查询支持的算法

响应体 [AlgorithmsResponse](file:///workspace/internal/model/api.go#L62-L64)：
```json
{
  "algorithms": [
    { "algorithm": "AES-128", "key_bits": 128, "modes": ["GCM","CBC","CTR","CFB","OFB"] },
    { "algorithm": "AES-192", "key_bits": 192, "modes": ["GCM","CBC","CTR","CFB","OFB"] },
    { "algorithm": "AES-256", "key_bits": 256, "modes": ["GCM","CBC","CTR","CFB","OFB"] },
    { "algorithm": "SM4",     "key_bits": 128, "modes": ["GCM","CBC","CTR","CFB","OFB"] }
  ]
}
```

## 安全设计要点

1. **根密钥保护**：SRK 私钥仅存在于 TPM 内部，代码中无任何导出逻辑
2. **数据密钥传输**：通过 TLS 加密的 HTTPS 通道传输 plaintext_data_key
3. **内存清零**：使用 `bytes.Fill` 或 `memclr` 模式将密钥使用后的内存清零
4. **密文认证**：GCM 模式下认证失败立即终止操作并返回 `ErrAuthFailed`
5. **篡改检测**：encrypted_data_key 被篡改时 TPM2_RSA_Decrypt 失败，返回 `ErrKeyDecryptFailed`
6. **密钥轮换**：支持定期轮换数据密钥，旧密钥解密的数据需重新加密
7. **TPM 模拟器支持**：开发环境支持 TPM 2.0 模拟器（swtpm），无需物理 TPM

## 数据格式规范

### 加密数据密钥密文格式（encrypted_data_key）
- 直接为 TPM2_RSA_Decrypt 可处理的二进制密文（RSA-OAEP with SHA-256）
- 在 JSON API 中以 base64 编码传输
- 大小：RSA-2048 时为 256 字节

### 业务数据密文格式（CiphertextFormat）[已实现](file:///workspace/internal/model/ciphertext.go#L7-L13)
```json
{
  "algorithm": "AES-256",
  "mode": "GCM",
  "iv": "base64(12字节随机数)",
  "tag": "base64(16字节认证标签)",
  "data": "base64(密文数据)"
}
```

## MODIFIED Requirements

### Requirement: internal/model API 类型扩展

现有 [api.go](file:///workspace/internal/model/api.go) 缺少解密数据密钥的请求/响应类型，需补充。

**新增**：`DecryptKeyRequest` 和 `DecryptKeyResponse`

## ADDED Requirements

### Requirement: TPM 2.0 上下文管理

系统 SHALL 提供 TPMContext 类型，管理与 TPM 设备的连接生命周期。

- **Open**：打开 TPM 设备（默认 `/dev/tpm0`，可配置），验证 TPM 2.0 能力
- **Close**：清理 TPM 句柄、关闭设备连接
- **失败处理**：TPM 不可用时返回 `ErrTPMNotAvailable`

#### Scenario: 首次打开 TPM
- **WHEN** `TPMContext.Open(devicePath)` 被调用且设备可用
- **THEN** 返回成功的 TPMContext，内部持有 TPM 读写通道

#### Scenario: TPM 设备缺失
- **WHEN** `TPMContext.Open(devicePath)` 被调用但设备不存在
- **THEN** 返回 `ErrTPMNotAvailable`

### Requirement: SRK 根密钥管理

系统 SHALL 提供 RootKey 类型，管理 TPM 内的 Storage Root Key。

- **CreateOrLoadSRK**：首次调用时在 Owner hierarchy 创建 RSA 2048 主密钥；后续调用加载已有密钥
- **GetPublicKey**：返回 RSA 公钥，供离线加密数据密钥使用
- **GetHandle**：返回 TPM 内部句柄，供解密操作使用

#### Scenario: 首次创建 SRK
- **WHEN** TPM 中不存在 SRK 且 `RootKey.CreateOrLoadSRK(ctx)` 被调用
- **THEN** 执行 `TPM2_CreatePrimary` 创建 RSA 2048 受限解密密钥，返回根密钥句柄和公钥

#### Scenario: 加载已有 SRK
- **WHEN** TPM 中已有 SRK 模板对应的持久化句柄
- **THEN** 直接加载，返回根密钥句柄和公钥

### Requirement: 数据密钥 TPM 内解密

系统 SHALL 使用 SRK 在 TPM 内解密受保护的数据密钥。

- **DecryptDataKey**：接收 encrypted_data_key，调用 `TPM2_RSA_Decrypt`（OAEP with SHA-256, empty label），返回明文密钥

#### Scenario: 正常解密
- **WHEN** 传入有效的 encrypted_data_key（由同一 SRK 加密）
- **THEN** 返回原始明文数据密钥字节

#### Scenario: 密文被篡改
- **WHEN** 传入被修改或不匹配 SRK 的 encrypted_data_key
- **THEN** TPM 解密失败，返回 `ErrKeyDecryptFailed`

### Requirement: 数据密钥软件加密（使用 SRK 公钥）

系统 SHALL 使用 SRK 的 RSA 公钥以 OAEP 模式加密新生成的数据密钥。

- **EncryptDataKey**：生成指定长度的随机 plaintext_data_key，用 SRK 公钥加密

#### Scenario: 生成并加密 AES-256 数据密钥
- **WHEN** 调用 `EncryptDataKey(algorithm=AES-256)`
- **THEN** 生成 32 字节随机密钥 + RSA-OAEP(SHA-256) 加密 + 返回(encrypted_key, plaintext_key)

#### Scenario: 生成并加密 SM4 数据密钥
- **WHEN** 调用 `EncryptDataKey(algorithm=SM4)`
- **THEN** 生成 16 字节随机密钥 + RSA-OAEP(SHA-256) 加密 + 返回(encrypted_key, plaintext_key)

### Requirement: KeyServer HTTP 服务

系统 SHALL 提供独立运行的密钥服务 HTTP 服务。

- **路由**：`/api/v1/keys/generate`, `/api/v1/keys/decrypt`, `/api/v1/keys/rotate`, `/api/v1/algorithms`
- **TLS**：支持配置证书，以 HTTPS 方式运行
- **健康检查**：`GET /health` 返回服务状态（含 TPM 连接状态）

#### Scenario: 服务启动
- **WHEN** `cmd/keyserver` 启动，读取配置（监听地址、TPM 路径、TLS 证书）
- **THEN** 初始化 TPMContext → 创建/加载 SRK → 启动 HTTP 服务 → 监听请求

#### Scenario: 健康检查
- **WHEN** `GET /health`
- **THEN** 返回 `{"status": "healthy", "tpm_available": true}`

### Requirement: EncryptionClient 客户端

系统 SHALL 提供业务节点集成使用的 EncryptionClient。

- **配置**：支持多个 KeyServer 地址，支持 TLS 证书配置
- **故障转移**：当前地址不可用时，自动尝试下一个地址
- **加密接口**：`Encrypt(plaintext, encryptedDataKey, algorithm, mode) → []byte(CiphertextFormat JSON)`
- **解密接口**：`Decrypt(ciphertextBytes, encryptedDataKey, algorithm, mode) → []byte(plaintext)`
- **密钥管理**：`GenerateKey(algorithm) → (encryptedKey, plaintextKey)`, `RotateKey(oldEncryptedKey, algorithm) → (newEncryptedKey, newPlaintextKey)`

#### Scenario: 客户端加密（首次，密钥不在内存）
- **WHEN** `client.Encrypt(data, encryptedKey, AES256, GCM)` 且缓存中无对应明文密钥
- **THEN** 请求 `/api/v1/keys/decrypt` 获取 plaintext_data_key → 缓存到内存 → 本地 AES-GCM 加密 → 返回 CiphertextFormat JSON

#### Scenario: 客户端加密（密钥已缓存）
- **WHEN** `client.Encrypt(data, encryptedKey, AES256, GCM)` 且缓存中有对应明文密钥
- **THEN** 直接使用缓存的明文密钥本地加密 → 返回 CiphertextFormat JSON

#### Scenario: 故障转移
- **WHEN** 客户端配置了 `[https://ks1.example.com, https://ks2.example.com]` 且 ks1 不可达
- **THEN** 自动尝试 ks2，若成功则切换；若全部失败则返回 `ErrKeyServerUnavailable`

### Requirement: 密钥内存安全

系统 SHALL 在数据密钥使用完毕后，将其在内存中的明文部分清零。

- `EncryptionClient` 在调用本地加解密完成后，对临时 plaintext_data_key 执行字节清零
- TPM 解密函数返回的明文密钥，使用完成后立即清零

### Requirement: 配置管理

系统 SHALL 提供配置加载能力，支持文件或环境变量两种方式。

KeyServer 配置：
- `TPMDevicePath`: TPM 设备路径，默认 `/dev/tpm0`
- `ListenAddr`: HTTP 监听地址，默认 `:8080`
- `TLSCertPath`, `TLSKeyPath`: TLS 证书与私钥路径，为空则使用 HTTP
- `ReadTimeout`, `WriteTimeout`: HTTP 超时设置

Client 配置：
- `KeyServerURLs`: 多个 KeyServer 地址（支持故障转移）
- `TLSCACertPath`: 验证 KeyServer 证书的 CA 根证书（可选）
- `InsecureSkipVerify`: 开发环境下可设为 true（生产禁用）

### Requirement: TPM 模拟器支持（开发环境）

系统 SHALL 在无物理 TPM 的开发环境中支持 TPM 2.0 模拟器。

- 使用 `swtpm` 或 `tpm2-simulator` 创建虚拟 TPM 设备
- TPMContext 可通过配置设备路径连接到模拟器

## REMOVED Requirements

无，本次为全新功能开发。
