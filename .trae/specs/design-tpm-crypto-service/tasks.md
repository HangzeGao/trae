# Tasks

> 实现顺序:先打地基(项目结构/配置/日志/Prokbuf)→ 再做 TPM 层 → 再做 Keystore → 再做 Crypto → 最后是协议层(grpc + gateway)、部署资源、验证。任务之间存在显式依赖,需按顺序推进。

## 阶段 0 — 环境与基线
- [ ] Task 0.1: 创建 `feature/tpm-crypto-service` 分支并初始化 Go 模块
  - [ ] SubTask 0.1.1: 在仓库根目录执行 `git checkout -b feature/tpm-crypto-service`
  - [ ] SubTask 0.1.2: 在 `tpm-crypto-service/` 下执行 `go mod init github.com/<org>/tpm-crypto-service`
  - [ ] SubTask 0.1.3: 确认 Go 版本 ≥ 1.22(支持 `log/slog` 备选,`crypto/subtle` 等)

## 阶段 1 — 项目骨架与公共能力
- [ ] Task 1.1: 规划目录结构与 `.gitignore` / `Makefile` / `Dockerfile`
  - [ ] SubTask 1.1.1: 目录按 spec 落地(`cmd/`,`internal/{tpm,keystore,crypto,service,server,config}`,`api/proto`,`deploy/`)
  - [ ] SubTask 1.1.2: `Makefile` 包含 `proto`, `build`, `test`, `lint`, `run` 目标
  - [ ] SubTask 1.1.3: Dockerfile 多阶段构建,运行镜像使用 `distroless/static` 或 `gcr.io/distroless/base-debian12`
- [ ] Task 1.2: 引入配置层(`internal/config`)
  - [ ] SubTask 1.2.1: 集成 `spf13/viper` + `pflag`,实现文件/env/etcd/CLI 优先级合并
  - [ ] SubTask 1.2.2: 定义 `Config` 结构体(嵌套 `server/tpm/keystore/deploy/rate_limit/log`),并通过单元测试覆盖优先级
  - [ ] SubTask 1.2.3: 默认 `config.example.yaml` 与 `docs/config.md`(后者仅在 spec 显式要求时创建)
- [ ] Task 1.3: 引入日志、指标、Tracing
  - [ ] SubTask 1.3.1: zap 结构化 JSON 日志,字段 `service`, `instance_id`, `tpm_mode`
  - [ ] SubTask 1.3.2: Prometheus 注册直方图 `crypto_op_duration_seconds{op,algorithm,mode,result}`
  - [ ] SubTask 1.3.3: 暴露 `/metrics` HTTP 端点(从 :8080)

## 阶段 2 — Protobuf 协议与代码生成
- [ ] Task 2.1: 编写 `api/proto/crypto.proto`
  - [ ] SubTask 2.1.1: 定义 `CryptoService` 7 个 RPC(见 spec 附录 A)
  - [ ] SubTask 2.1.2: 定义枚举 `Algorithm(AES,SM4)`、`BlockMode(ECB,CBC,CTR,CFB,OFB,GCM)`、`KeyStatus`
  - [ ] SubTask 2.1.3: 用 `google.api.http` 注解挂载 HTTP 路由
- [ ] Task 2.2: 配置 `buf` 生成 gRPC + grpc-gateway stub
  - [ ] SubTask 2.2.1: 引入 `buf.gen.yaml`,生成 `*.pb.go` + `*_grpc.pb.go` + `*.pb.gw.go`
  - [ ] SubTask 2.2.2: 通过 `make proto` 一键生成,并把生成物提交到仓库(便于审阅)

## 阶段 3 — TPM 2.0 抽象层
- [ ] Task 3.1: 封装 `internal/tpm/device`
  - [ ] SubTask 3.1.1: `Open(opts)` 根据 `device_path`/`simulator` 返回 `*TPM`
  - [ ] SubTask 3.1.2: 暴露 `Close()`,`Health()`(`TPM2_GetCapability` + 自检)
  - [ ] SubTask 3.1.3: 错误包装为 `codes.Unavailable` / `codes.FailedPrecondition`
- [ ] Task 3.2: 实现 SRK / ARK 生命周期
  - [ ] SubTask 3.2.1: `EnsureSRK()` — 调用 `TPM2_CreatePrimary`(RSA 2048 SRK 模板或 ECC 模板,二选一,需注释说明)
  - [ ] SubTask 3.2.2: `SealARK(plaintext, pcrs)` — 创建 AES-256 密钥 → `TPM2_Create` → `TPM2_Load` 到 NULL hierarchy → 输出密文 blob
  - [ ] SubTask 3.2.3: `UnsealARK(sealedBlob, pcrs)` — 反向操作,PCR 不匹配时返回 `codes.PermissionDenied`
  - [ ] SubTask 3.2.4: 单元测试使用 `go-tpm-tools/simulator` 跑通 Seal/Unseal 往返

## 阶段 4 — 密钥库
- [ ] Task 4.1: 定义 `internal/keystore` 接口
  - [ ] SubTask 4.1.1: `Get(ctx, keyID) (WrappedDEK, error)` / `Put(ctx, keyID, WrappedDEK) error` / `List(ctx) ([]DataKeyMeta, error)` / `Delete(ctx, keyID) error`
  - [ ] SubTask 4.1.2: `WrappedDEK` 字段:`{Algorithm, KeyLengthBits, Version, WrappedKey(GCM密文+nonce+tag), CreatedAt, Status}`
- [ ] Task 4.2: 实现 BoltDB 后端
  - [ ] SubTask 4.2.1: 使用 `go.etcd.io/bbolt`,Bucket 设计 `meta` 与 `blob`
  - [ ] SubTask 4.2.2: 集成测试覆盖并发读写、删除幂等
- [ ] Task 4.3: 实现 etcd 后端
  - [ ] SubTask 4.3.1: 封装 `clientv3`,路径 `/tpm-crypto/dek/<keyID>/<version>` 与 `/tpm-crypto/dek/<keyID>/meta`
  - [ ] SubTask 4.3.2: 通过容器化集成测试(docker-compose 起 etcd)验证

## 阶段 5 — 算法与分组模式层
- [ ] Task 5.1: 抽象 `internal/crypto` 注册表
  - [ ] SubTask 5.1.1: `Cipher` 接口 `Encrypt(plain, iv, aad) (cipher, tag, error)` / `Decrypt(...)`
  - [ ] SubTask 5.1.2: 按 `(Algorithm, BlockMode)` 路由到实现;不支持组合返回 `codes.InvalidArgument`
- [ ] Task 5.2: AES 实现
  - [ ] SubTask 5.2.1: GCM / CTR / CFB / OFB / CBC(encrypt-then-MAC) / ECB(仅限单块)
  - [ ] SubTask 5.2.2: 所有 IV/nonce 通过 `crypto/rand` 生成并随密文返回
- [ ] Task 5.3: SM4 实现(`gmsm/sm4`)
  - [ ] SubTask 5.3.1: GCM / CTR / CFB / OFB / CBC(encrypt-then-MAC) / ECB(仅限单块)
  - [ ] SubTask 5.3.2: 与 AES 路径并行,使用同一 `Cipher` 接口
- [ ] Task 5.4: HMAC-SHA256 完整性绑定(CBC 模式)
  - [ ] SubTask 5.4.1: 派生子密钥 `K_enc||K_mac`,通过 HKDF 派生
  - [ ] SubTask 5.4.2: 加密时先密文后 MAC,解密时先校验 MAC 再解密(`subtle.ConstantTimeCompare`)

## 阶段 6 — 业务用例层
- [ ] Task 6.1: `internal/service/crypto.go`
  - [ ] SubTask 6.1.1: `CreateDataKey` / `DescribeDataKey` / `RotateDataKey` / `RevokeDataKey`
  - [ ] SubTask 6.1.2: `Encrypt` / `Decrypt`(从 keystore 取出 WrappedDEK → ARK 解封 DEK → 调用 crypto 层)
  - [ ] SubTask 6.1.3: 全程审计日志 + Prometheus 指标埋点

## 阶段 7 — 协议层
- [ ] Task 7.1: gRPC + HTTP Gateway 服务
  - [ ] SubTask 7.1.1: `internal/server/grpc.go` — 监听 :9090,挂载拦截器(unary: zap 日志 + prom 指标 + 限流)
  - [ ] SubTask 7.1.2: `internal/server/http.go` — 监听 :8080,挂载 grpc-gateway runtime mux + `/healthz` + `/metrics`
  - [ ] SubTask 7.1.3: mTLS 可选(`server.tls.{cert,key,ca}`),未配置时回退为明文
- [ ] Task 7.2: 优雅关停
  - [ ] SubTask 7.2.1: `signal.NotifyContext(SIGTERM,SIGINT)` 触发 `grpc.GracefulStop` + `http.Server.Shutdown(ctx)`
  - [ ] SubTask 7.2.2: 30 秒超时内未完成则强制退出码 1

## 阶段 8 — 部署资源
- [ ] Task 8.1: 隔离部署
  - [ ] SubTask 8.1.1: `deploy/isolated/systemd/tpm-crypto.service`
  - [ ] SubTask 8.1.2: `deploy/isolated/config.yaml`(keystore=bolt, simulator=false)
- [ ] Task 8.2: 集群部署
  - [ ] SubTask 8.2.1: `deploy/cluster/k8s/deployment.yaml`(多副本,readiness 指向 `/healthz`)
  - [ ] SubTask 8.2.2: `deploy/cluster/k8s/service.yaml` + `ingress.yaml`(gRPC + HTTP)
  - [ ] SubTask 8.2.3: `deploy/cluster/helm/tpm-crypto-service/` 提供可参数化的 Chart
  - [ ] SubTask 8.2.4: `deploy/cluster/compose/docker-compose.yaml`(含 etcd、tpm-simulator、service)用于本地端到端

## 阶段 9 — 验证
- [ ] Task 9.1: 单元测试覆盖率 ≥ 70%
  - [ ] SubTask 9.1.1: `internal/crypto` 每个 `(algo,mode)` 至少一个 round-trip 测试向量
  - [ ] SubTask 9.1.2: `internal/tpm` 在 simulator 下覆盖 Seal/Unseal、PCR 拒绝
  - [ ] SubTask 9.1.3: `internal/service` 使用 mock keystore + mock TPM 跑通 7 个 RPC
- [ ] Task 9.2: 端到端测试
  - [ ] SubTask 9.2.1: `docker-compose up` 启 etcd + tpm-simulator + service
  - [ ] SubTask 9.2.2: 用 `grpcurl` 与 `curl` 同时打 7 个 RPC,断言响应一致
  - [ ] SubTask 9.2.3: 故障注入: kill service,重启后 ARK 自动恢复,DEK 可继续解密
- [ ] Task 9.3: 安全/合规自检
  - [ ] SubTask 9.3.1: `gosec`/`govet`/`staticcheck` 通过
  - [ ] SubTask 9.3.2: 确认日志中无密钥明文、ARGC/DEK、ARK
  - [ ] SubTask 9.3.3: 文档列明「禁止 ECB 业务使用」,代码层在 ECB + >1 块时拒绝

---

# Task Dependencies
- [Task 1.1, 1.2, 1.3] 依赖 [Task 0.1]
- [Task 2.1, 2.2] 依赖 [Task 1.1]
- [Task 3.1, 3.2] 依赖 [Task 1.1, 2.1] (TPM 错误码需要与 proto 一致)
- [Task 4.1] 依赖 [Task 1.1]
- [Task 4.2, 4.3] 依赖 [Task 4.1]
- [Task 5.1] 依赖 [Task 1.1]
- [Task 5.2, 5.3, 5.4] 依赖 [Task 5.1]
- [Task 6.1] 依赖 [Task 3.2, 4.x, 5.x]
- [Task 7.1, 7.2] 依赖 [Task 6.1, 2.2]
- [Task 8.x] 依赖 [Task 7.x]
- [Task 9.x] 依赖 [Task 8.x](或并行,Task 9.1 单元测试不依赖部署)
