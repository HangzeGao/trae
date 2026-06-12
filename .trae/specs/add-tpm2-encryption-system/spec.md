# 基于 TPM 2.0 的软硬结合加解密系统 Spec

## Why
当前系统已实现 AES/SM4 多模式对称加解密能力，但缺乏 TPM 2.0 硬件安全集成和集群部署支持。数据密钥以明文形式在内存中流转，无法在多服务器间安全分发，且不支持隔离部署场景。需要引入 TPM 2.0 根密钥保护数据密钥，并支持密钥服务独立部署，使多业务节点能安全共享加解密能力。

## What Changes
- 新增 TPM 2.0 根密钥管理模块：在 TPM 芯片内创建/加载 SRK，私钥永不离开 TPM
- 新增数据密钥管理模块：生成随机数据密钥，使用根密钥在 TPM 内加密，仅存储/传输密文
- 新增密钥服务（KeyServer）：独立 HTTP 服务，封装 TPM 密钥操作，支持隔离部署
- 新增加解密客户端（EncryptionClient）：业务节点通过客户端与密钥服务交互，本地执行加解密
- 新增集群部署支持：多密钥服务实例通过共享加密数据密钥实现跨节点加解密
- 补充 HTTP API 服务层：基于已有 model/api.go 定义实现完整 REST API
- **BREAKING** 无，此为全新功能

## Impact
- Affected specs: 无已有规格受影响
- Affected code: 复用已有 `internal/crypto` 和 `internal/model` 包，新增 `internal/tpm`、`internal/keyserver`、`internal/client`、`cmd/` 等模块

## ADDED Requirements

### Requirement: TPM 2.0 根密钥管理
系统 SHALL 在 TPM 2.0 芯片内创建和管理根密钥（Storage Root Key），根密钥的私钥部分永不离开 TPM 芯片。

#### Scenario: 首次初始化根密钥
- **WHEN** 密钥服务首次启动且 TPM 中不存在根密钥
- **THEN** 系统在 TPM 中创建新的 SRK，返回根密钥句柄，服务进入就绪状态

#### Scenario: 加载已有根密钥
- **WHEN** 密钥服务启动且 TPM 中已存在根密钥
- **THEN** 系统加载已有根密钥并返回句柄，不重复创建

#### Scenario: TPM 不可用
- **WHEN** TPM 设备不可用或驱动异常
- **THEN** 系统返回 `ErrTPMNotAvailable` 错误，拒绝继续操作

### Requirement: 数据密钥管理
系统 SHALL 支持数据密钥的生成、加密存储和解密加载，数据密钥由根密钥在 TPM 内加密保护。

#### Scenario: 生成数据密钥
- **WHEN** 客户端请求生成新的数据密钥，指定算法类型
- **THEN** 系统生成对应长度的随机数据密钥，使用根密钥在 TPM 内加密数据密钥，返回加密后的数据密钥密文和明文数据密钥（仅本次返回）

#### Scenario: 解密数据密钥
- **WHEN** 客户端需要使用已加密的数据密钥
- **THEN** 系统将加密的数据密钥密文送入 TPM，由根密钥解密，返回明文数据密钥

#### Scenario: 数据密钥密文被篡改
- **WHEN** 数据密钥密文被篡改或与根密钥不匹配
- **THEN** TPM 解密失败，系统返回 `ErrKeyDecryptFailed` 错误

### Requirement: 多算法多模式加解密
系统 SHALL 支持 AES-128/192/256 和 SM4 算法，以及 GCM/CBC/CTR/CFB/OFB 分组模式。

#### Scenario: AES-256-GCM 加解密
- **WHEN** 客户端指定 AES-256 算法和 GCM 模式进行加解密
- **THEN** 系统使用 32 字节密钥以 AES-256-GCM 加解密，密文包含 IV 和认证标签

#### Scenario: SM4-GCM 加解密
- **WHEN** 客户端指定 SM4 算法和 GCM 模式进行加解密
- **THEN** 系统使用 16 字节密钥以 SM4-GCM 加解密，密文包含 IV 和认证标签

#### Scenario: 不支持的算法或模式
- **WHEN** 客户端指定不支持的算法或模式
- **THEN** 系统返回 `ErrInvalidAlgorithm` 或 `ErrInvalidMode` 错误

### Requirement: 密钥服务（KeyServer）隔离部署
系统 SHALL 支持密钥服务作为独立 HTTP 服务部署，与业务节点物理隔离。

#### Scenario: 密钥服务独立启动
- **WHEN** 以 KeyServer 模式启动服务
- **THEN** 服务监听配置的 HTTP 端口，提供密钥生成/解密/轮换等 API，TPM 操作仅在密钥服务节点执行

#### Scenario: 密钥服务不可达
- **WHEN** 业务节点的客户端无法连接密钥服务
- **THEN** 客户端返回 `ErrKeyServerUnavailable` 错误，业务节点可降级处理

#### Scenario: 密钥服务 TLS 通信
- **WHEN** 密钥服务配置了 TLS 证书
- **THEN** 客户端与密钥服务之间通过 TLS 加密通信，防止数据密钥明文在网络中泄露

### Requirement: 多服务器/集群部署
系统 SHALL 支持多业务节点共享同一密钥服务或多个密钥服务实例的集群部署。

#### Scenario: 多业务节点共享密钥服务
- **WHEN** 多个业务节点配置同一个密钥服务地址
- **THEN** 所有节点可通过同一密钥服务解密同一加密数据密钥，实现跨节点数据共享

#### Scenario: 密钥服务多实例部署
- **WHEN** 部署多个密钥服务实例（共享同一 TPM 或使用相同的 SRK）
- **THEN** 任一实例生成的加密数据密钥可被其他实例解密，客户端可配置多个服务地址实现高可用

#### Scenario: 客户端故障转移
- **WHEN** 客户端配置了多个密钥服务地址，且当前服务不可用
- **THEN** 客户端自动尝试下一个服务地址，直到成功或全部失败

### Requirement: 加解密客户端（EncryptionClient）
系统 SHALL 提供加解密客户端，业务节点通过客户端与密钥服务交互，本地执行加解密运算。

#### Scenario: 客户端初始化
- **WHEN** 创建 EncryptionClient 并配置密钥服务地址
- **THEN** 客户端验证服务可达性，进入就绪状态

#### Scenario: 一站式加密
- **WHEN** 调用 `client.Encrypt(plaintext, encryptedDataKey, algorithm, mode)`
- **THEN** 客户端先请求密钥服务解密数据密钥，再本地使用数据密钥执行加解密，返回密文

#### Scenario: 一站式解密
- **WHEN** 调用 `client.Decrypt(ciphertext, encryptedDataKey, algorithm, mode)`
- **THEN** 客户端先请求密钥服务解密数据密钥，再本地使用数据密钥执行解密，返回明文

### Requirement: 密钥轮换
系统 SHALL 支持数据密钥的轮换，旧密钥解密新密钥加密的数据迁移。

#### Scenario: 轮换数据密钥
- **WHEN** 客户端请求轮换数据密钥，提供旧加密数据密钥
- **THEN** 系统生成新数据密钥并用根密钥加密，返回新加密数据密钥和明文

#### Scenario: 数据重加密
- **WHEN** 需要用新密钥重新加密已有数据
- **THEN** 客户端使用旧密钥解密数据，再用新密钥重新加密

### Requirement: HTTP API
系统 SHALL 基于 REST API 提供密钥管理和加解密操作接口。

#### Scenario: 生成数据密钥
- **WHEN** POST /api/v1/keys/generate
- **THEN** 返回加密数据密钥和明文数据密钥

#### Scenario: 解密数据密钥
- **WHEN** POST /api/v1/keys/decrypt
- **THEN** 返回明文数据密钥

#### Scenario: 轮换数据密钥
- **WHEN** POST /api/v1/keys/rotate
- **THEN** 返回新加密数据密钥和明文数据密钥

#### Scenario: 查询支持的算法
- **WHEN** GET /api/v1/algorithms
- **THEN** 返回所有支持的算法和模式列表
