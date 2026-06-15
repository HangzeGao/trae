# TPM 2.0 软硬结合加解密服务 Spec

## Why
构建一个基于 TPM 2.0 硬件保护的高安全加解密服务,通过两层密钥体系(Root Key 保护 Data Key)将密钥材料与硬件可信根绑定,防止密钥泄露;同时通过 gRPC + HTTP 双协议网关对外提供对称加密能力,覆盖 AES 与国密 SM4,适配多种分组模式以满足不同业务场景的安全合规要求。

## What Changes
- 新建 Go 项目 `tpm-crypto-service`,使用 `grpc-go` + `grpc-gateway` 作为最经典、权威的双协议服务框架
- 集成 `github.com/google/go-tpm` 操作 TPM 2.0 设备(Simulator / Real Hardware)
- 集成 `github.com/tjfoc/gmsm`(国密标准库)提供 SM4 算法,使用 Go 标准库 `crypto/aes` 提供 AES 算法
- 实现两层密钥分层: 根密钥(Root Key)经 TPM Seal 保护 → 数据密钥(Data Key)经 Root Key 加密落盘
- 支持 AES-128/192/256 与 SM4-128 分组密码
- 支持 ECB / CBC / CTR / CFB / OFB / GCM 等主流安全分组模式(GCM 默认推荐)
- 支持隔离部署(单机单实例)与多机集群部署(无状态服务 + 共享外部密钥库)
- 新建 `feature/tpm-crypto-service` 分支作为开发基线

## Impact
- Affected specs: 新建 spec(本文件),无既有规范需要修改
- Affected code: 新建仓库 `tpm-crypto-service/` 下的所有代码文件
- Affected deployments: 部署环境需提供 TPM 2.0 设备(物理/虚拟 Simulator);多机部署需对接外部持久化(KV/DB)用于密文 Data Key 存储
- 关键路径:
  - `cmd/server` — 入口二进制
  - `internal/tpm` — TPM 2.0 抽象层
  - `internal/keystore` — 密钥存储与版本管理
  - `internal/crypto` — 算法与模式适配层
  - `internal/service` — 业务用例
  - `internal/server` — gRPC + HTTP(Gateway)服务器
  - `internal/config` — 多源配置(文件/环境变量/etcd)
  - `api/proto` — Protobuf 协议定义
  - `deploy/` — 隔离部署与集群部署资源

## 技术选型(已确定)

| 关注点 | 选型 | 选择理由 |
| --- | --- | --- |
| 服务框架 | `grpc-go` + `grpc-gateway` | Google 官方维护,业界最经典权威;同一份 Protobuf 同时暴露 gRPC 与 HTTP/JSON,降低双协议维护成本 |
| TPM 2.0 客户端 | `github.com/google/go-tpm` | Google 官方维护,支持 TPM 2.0 全部命令集;同源项目 `go-tpm-tools` 提供高级封装 |
| 国密库 | `github.com/tjfoc/gmsm` | 商用密码检测认证的开源实现,工业界部署最广 |
| AES | `crypto/aes` (Go 标准库) | FIPS-197 官方实现,经过 Go 团队与社区长期审计 |
| 日志 | `go.uber.org/zap` | 高性能结构化日志,生产首选 |
| 配置 | `spf13/viper` | 支持文件/环境变量/etcd 多源,与 `pflag` 集成 |
| Metrics | `prometheus/client_golang` | 云原生事实标准 |
| Tracing(可选) | `go.opentelemetry.io/otel` | OTLP 标准,便于接入 Jaeger/Tempo |
| 错误处理 | `github.com/grpc-ecosystem/grpc-gateway/v2` 自带 status 映射 | 与 gRPC 错误码一致 |
| 测试 | `github.com/stretchr/testify` + `github.com/google/go-tpm-tools/simulator` | 单元/集成测试可走 TPM Simulator,无需硬件即可 CI |

## 架构概览

```
+----------------------------+        +-----------------------------+
|       HTTP Client          |        |       gRPC Client           |
+-------------↑--------------+        +-------------↑---------------+
              | HTTP/JSON                          | Protobuf
              |                                    |
+-------------+--------+   gRPC-Gateway    +-------+-----------------+
|   HTTP Gateway :8080 |◄────────────────►|   gRPC Server :9090      |
+----------------------+                  +-------------+------------+
                                                        |
                          +-----------------------------+
                          |   Service Layer (Use Cases) |
                          +-------------+---------------+
                                        |
            +---------------+-----------+-----------+---------------+
            |               |                       |               |
   +--------▼-------+ +-----▼------+      +---------▼-----+ +-------▼-------+
   |  Keystore Mgr  | | Crypto Mgr |      |  TPM Wrapper  | |   Audit Log   |
   +--------+-------+ +-----+------+      +---------+-----+ +-------+-------+
            |               |                       |               |
   +--------▼-------+ +-----▼------+      +---------▼-----+ +-------▼-------+
   | External KV/DB | | AES / SM4  |      |  TPM Device   | |  zap → stdout |
   | (BoltDB/etcd)  | | ECB/CBC/.. |      |  (HW/Sim)     | |  or file      |
   +----------------+ +------------+      +---------------+ +---------------+
```

## 密钥分层模型

```
┌──────────────────────────────────────────────────────────────┐
│  Tier 0: TPM Storage Root Key (SRK)                          │
│          TPM 芯片内部,不可导出,仅用于 Seal/Unseal            │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Tier 1: Application Root Key (ARK)                    │  │
│  │          由 TPM SRK Seal 后落盘,仅在内存中以明文存在   │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  Tier 2: Data Encryption Keys (DEK)              │  │  │
│  │  │          明文存在内存中;落盘形态为 ARK 加密的密文 │  │  │
│  │  │  支持多版本轮转                                   │  │  │
│  │  │  ┌──────────────────────────────────────────────┐  │  │  │
│  │  │  │  业务数据(明文/密文)                         │  │  │  │
│  │  │  │  使用 DEK 明文 + 指定算法/模式 进行加解密    │  │  │  │
│  │  │  └──────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## ADDED Requirements

### Requirement: TPM 2.0 设备访问抽象
系统 SHALL 提供对 TPM 2.0 设备的统一访问抽象,支持物理 TPM(通过 `/dev/tpm0` 或 `/dev/tpmrm0`)与软件模拟器(`go-tpm-tools/simulator`)两种模式,通过配置切换。

#### Scenario: 启动连接真实 TPM
- **WHEN** 配置 `tpm.device_path=/dev/tpm0` 且设备存在
- **THEN** 服务成功建立 TPM 上下文,后续 Seal/Unseal 操作通过真实芯片执行
- **AND** 启动日志输出 `tpm device opened: /dev/tpm0`

#### Scenario: 启动连接软件模拟器
- **WHEN** 配置 `tpm.simulator=true` 或设备未找到
- **THEN** 服务启动内置 TPM 模拟器,所有 TPM 命令在内存中执行
- **AND** 启动日志明确标注 `tpm mode: simulator` 防止误用于生产

### Requirement: 应用根密钥(ARK)生命周期
系统 SHALL 在首次启动时自动创建 SRK(若不存在),并生成或恢复 ARK;ARK 通过 TPM 2.0 `Seal` 命令绑定到当前 PCR 状态(如 `PCR0:7`),落盘形态为 TPM 输出的密文 blob。

#### Scenario: 首次启动创建 ARK
- **WHEN** 系统中不存在已封存的 ARK
- **THEN** 自动生成 256-bit AES 随机密钥作为 ARK
- **AND** 通过 TPM `Seal(ARK, PCRs=current0)` 封存,得到 SealedBlob
- **AND** 将 SealedBlob 持久化到外部存储

#### Scenario: 重启恢复 ARK
- **WHEN** 已存在 SealedBlob 且当前 PCR 与封存时一致
- **THEN** 通过 TPM `Unseal(SealedBlob)` 还原 ARK 明文至内存
- **AND** ARK 在内存中受 `crypto/subtle` 约束使用,Go GC 不直接回收

#### Scenario: PCR 不匹配拒绝解封
- **WHEN** PCR 状态变化(例如引导链被篡改)
- **THEN** TPM 返回 unseal 错误,服务进入 fail-closed 模式
- **AND** 错误码 `PERMISSION_DENIED` 透传给调用方

### Requirement: 数据密钥(DEK)管理
系统 SHALL 提供 DEK 的创建、查询、轮转(rotate)、吊销(revoke)操作;DEK 落盘形态为 ARK 加密后的密文与元数据(算法、模式、创建时间、版本、状态)。

#### Scenario: 创建 DEK
- **WHEN** 客户端请求 `CreateDataKey` 并指定 `key_id`、算法(AES/SM4)、长度
- **THEN** 系统生成符合长度要求的随机密钥
- **AND** 使用 ARK 在 GCM 模式(AAD = key_id)下加密 DEK
- **AND** 持久化 `{key_id, algorithm, mode=GCM, wrapped_key, version=1, status=active, created_at}`

#### Scenario: 查询 DEK 元数据
- **WHEN** 客户端请求 `DescribeDataKey(key_id)`
- **THEN** 返回元数据但**不返回**明文密钥
- **AND** 包含 version、status、algorithm、created_at

#### Scenario: 轮转 DEK
- **WHEN** 客户端请求 `RotateDataKey(key_id)`
- **THEN** 旧 DEK 标记 `status=retired`(可解密历史数据)
- **AND** 生成新 DEK,version 自增,status=`active`
- **AND** 调用方获得新 version,旧 version 仍可用于解密

### Requirement: 对称加解密
系统 SHALL 提供 `Encrypt` / `Decrypt` 接口,支持 AES(128/192/256)与 SM4(128)算法,以及 ECB / CBC / CTR / CFB / OFB / GCM 分组模式。

#### Scenario: AES-GCM 加密
- **WHEN** 客户端请求 `Encrypt(plaintext, key_id, algorithm=AES, mode=GCM, aad)`
- **THEN** 系统按 key_id 加载 DEK 明文
- **AND** 使用 AES-GCM,nonce 由系统安全随机生成
- **AND** 输出 `{ciphertext, nonce, tag, key_version}` 给客户端
- **AND** 不在响应中暴露 DEK 明文

#### Scenario: AES-CBC 加密(含 HMAC-SHA256 校验)
- **WHEN** 客户端请求 `Encrypt(plaintext, key_id, mode=CBC)`
- **THEN** 使用 AES-CBC 加密,IV 由系统安全随机生成
- **AND** 附加 HMAC-SHA256(encrypt-then-MAC)以保证完整性
- **AND** 输出 `{ciphertext, iv, mac, key_version}`

#### Scenario: SM4-GCM 加密
- **WHEN** 客户端请求 `Encrypt(plaintext, key_id, algorithm=SM4, mode=GCM)`
- **THEN** 使用 `gmsm/sm4` 包 GCM 模式加密
- **AND** 输出格式与 AES-GCM 保持一致字段

#### Scenario: 模式不匹配返回 INVALID_ARGUMENT
- **WHEN** 请求的算法与模式组合不被支持(例如 SM4-192、SM4-ECB-GCM)
- **THEN** 立即返回 `INVALID_ARGUMENT` 错误并指明原因

### Requirement: 协议层(gRPC + HTTP)
系统 SHALL 同时暴露 gRPC(:9090)与 HTTP/JSON(:8080, 由 grpc-gateway 反向代理)两类端点,使用同一份 Protobuf 定义。

#### Scenario: 客户端通过 gRPC 调用
- **WHEN** 客户端使用 Protobuf stub 连接 `:9090`
- **THEN** 所有方法按 Protobuf 行为正确响应

#### Scenario: 客户端通过 HTTP 调用
- **WHEN** 客户端 `POST /v1/encrypt` 发送 JSON
- **THEN** 请求被 grpc-gateway 转译为 gRPC 调用并返回 JSON 响应
- **AND** HTTP 状态码与 gRPC 错误码按官方映射表一致(例如 `NOT_FOUND → 404`)

### Requirement: 服务框架基础设施
系统 SHALL 提供启动/优雅关停、配置加载、结构化日志、Prometheus 指标、健康检查、TLS 等生产级能力。

#### Scenario: 优雅启动
- **WHEN** 启动二进制 `tpm-crypto-service --config config.yaml`
- **THEN** 加载 viper 配置、初始化 zap logger、初始化 TPM、初始化 keystore
- **AND** 监听 gRPC :9090 与 HTTP :8080
- **AND** 启动耗时上报为 Prometheus 直方图 `service_start_duration_seconds`

#### Scenario: 健康检查
- **WHEN** 探针调用 `GET /healthz`(HTTP)或 `Health.Check`(gRPC)
- **THEN** 返回 `SERVING` 当且仅当 TPM 可达、ARK 已解封、keystore 可读
- **AND** 任意子项失败返回 `NOT_SERVING` 与失败原因

#### Scenario: 优雅关停
- **WHEN** 进程收到 SIGTERM/SIGINT
- **THEN** 30 秒内完成:停止接收新请求 → 排空 in-flight → 关闭 TPM 上下文 → 退出码 0

### Requirement: 部署模式
系统 SHALL 支持「隔离部署」与「多机/集群部署」两种模式,使用相同的二进制,仅通过配置区分。

#### Scenario: 隔离部署(单机)
- **WHEN** 配置 `deploy.mode=isolated`,`keystore.backend=bolt`,`keystore.path=/var/lib/tpm-crypto/data.bolt`
- **THEN** 服务使用本地 BoltDB 存储 wrapped DEK,TPM 设备由本机提供
- **AND** 适合单节点开发、PoC、硬件强一致场景

#### Scenario: 多机/集群部署
- **WHEN** 配置 `deploy.mode=cluster`,`keystore.backend=etcd`,`keystore.endpoints=etcd1:2379,etcd2:2379`
- **THEN** 服务以无状态方式运行,wrapped DEK 存储在 etcd 集群
- **AND** 每个节点需拥有等价 PCR 状态的 TPM(或使用共同外部 HSM 代理)
- **AND** 支持横向扩缩容,多副本通过 keepalive+lb 对外提供

#### Scenario: 配置优先级
- **WHEN** 同时存在配置文件、环境变量、远程配置中心(etcd/Consul)
- **THEN** 优先级为:`CLI flag > 环境变量 > 远程配置 > 配置文件 > 默认值`

### Requirement: 安全与审计
系统 SHALL 记录所有密钥操作(`CreateDataKey` / `RotateDataKey` / `RevokeDataKey` / `Encrypt` / `Decrypt`)的审计日志,记录 key_id、调用方、算法/模式、时间戳、是否成功。

#### Scenario: 审计日志结构化输出
- **WHEN** 任一 API 被调用
- **THEN** 写入 zap 日志字段 `audit=true`,包含 `caller`、`method`、`key_id`、`algorithm`、`mode`、`result=ok|err`
- **AND** 敏感字段(明文、DEK、ARK)严禁出现在日志

#### Scenario: 速率限制
- **WHEN** 同一 client 超过 `rate_limit.qps`(默认 1000 QPS)
- **THEN** 返回 `RESOURCE_EXHAUSTED` 错误
- **AND** 限流指标 `crypto_rate_limited_total` 自增

### Requirement: 协议与错误约定
系统 SHALL 严格遵循 gRPC 错误码语义并通过 grpc-gateway 正确映射为 HTTP 状态码。

#### Scenario: 错误码映射
- **WHEN** 业务层返回 `codes.NotFound`
- **THEN** HTTP 响应状态码为 404
- **WHEN** 业务层返回 `codes.PermissionDenied`
- **THEN** HTTP 响应状态码为 403
- **WHEN** 业务层返回 `codes.Internal`
- **THEN** HTTP 响应状态码为 500,且 body 仅包含脱敏后的 `message`

## MODIFIED Requirements
无(本项目为全新模块,无既有规范被修改)。

## REMOVED Requirements
无。

## 附录 A — API 定义(摘要)

```protobuf
service CryptoService {
  rpc CreateDataKey(CreateDataKeyRequest) returns (DataKeyMeta);
  rpc DescribeDataKey(DescribeDataKeyRequest) returns (DataKeyMeta);
  rpc RotateDataKey(RotateDataKeyRequest) returns (DataKeyMeta);
  rpc RevokeDataKey(RevokeDataKeyRequest) returns (RevokeDataKeyResponse);
  rpc Encrypt(EncryptRequest) returns (EncryptResponse);
  rpc Decrypt(DecryptRequest) returns (DecryptResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message DataKeyMeta {
  string key_id = 1;
  Algorithm algorithm = 2;     // AES / SM4
  uint32 key_length_bits = 3;  // 128 / 192 / 256
  uint64 version = 4;
  KeyStatus status = 5;        // ACTIVE / RETIRED / REVOKED
  google.protobuf.Timestamp created_at = 6;
}

message EncryptRequest {
  bytes plaintext = 1;
  string key_id = 2;
  Algorithm algorithm = 3;
  BlockMode mode = 4;          // ECB / CBC / CTR / CFB / OFB / GCM
  bytes aad = 5;               // 用于 GCM 等 AEAD 模式
  uint64 key_version = 6;      // 留空使用最新 active
}
```

## 附录 B — 关键安全约束

1. **永不落盘 ARK / DEK 明文** — 仅密文落盘
2. **DEK 加密必须用 AEAD(GCM)** — 防篡改且性能可接受
3. **CBC 必须配 HMAC(encrypt-then-MAC)** — 不接受裸 CBC
4. **禁止使用 ECB 处理 >1 块数据** — 业务侧校验并在文档中标注
5. **TPM 上下文超时自动重连** — 防止长连接被对端关闭
6. **所有随机数使用 `crypto/rand`** — 禁止使用 `math/rand`
7. **结构体中密钥字段使用 `[]byte` + 显式清零** — 用 `subtle.ConstantTimeCompare` 比较

## 附录 C — 配置示例(节选)

```yaml
server:
  grpc_addr: ":9090"
  http_addr: ":8080"
  shutdown_timeout: 30s

tpm:
  device_path: "/dev/tpm0"
  simulator: false           # 物理 TPM 不可用时置 true
  pcr_selection: [0,1,2,3,4,5,6,7]

keystore:
  backend: "bolt"            # bolt | etcd
  path: "/var/lib/tpm-crypto/data.bolt"
  # etcd 模式:
  # backend: "etcd"
  # endpoints: ["etcd-1:2379","etcd-2:2379","etcd-3:2379"]

deploy:
  mode: "isolated"           # isolated | cluster

rate_limit:
  qps: 1000
  burst: 2000

log:
  level: "info"
  encoding: "json"
  output: "stdout"
```
