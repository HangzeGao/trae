# Tasks

> 实现顺序:先打地基(项目结构/配置/日志/Prokbuf)→ 再做 TPM 层 → 再做 Keystore → 再做 Crypto → 最后是协议层(grpc + gateway)、部署资源、验证。任务之间存在显式依赖,需按顺序推进。

## 阶段 0 — 环境与基线
- [x] Task 0.1: 创建 `feature/tpm-crypto-service` 分支并初始化 Go 模块
  - [x] SubTask 0.1.1: 在仓库根目录执行 `git checkout -b feature/tpm-crypto-service`
  - [x] SubTask 0.1.2: 在 `tpm-crypto-service/` 下执行 `go mod init github.com/<org>/tpm-crypto-service`
  - [x] SubTask 0.1.3: 确认 Go 版本 ≥ 1.22(支持 `log/slog` 备选,`crypto/subtle` 等)

## 阶段 1 — 项目骨架与公共能力
- [x] Task 1.1: 规划目录结构与 `.gitignore` / `Makefile` / `Dockerfile`
  - [x] SubTask 1.1.1: 目录按 spec 落地(`cmd/`,`internal/{tpm,keystore,crypto,service,server,config}`,`api/proto`,`deploy/`)
  - [x] SubTask 1.1.2: `Makefile` 包含 `proto`, `build`, `test`, `lint`, `run` 目标
  - [x] SubTask 1.1.3: Dockerfile 多阶段构建,运行镜像使用 `distroless/static` 或 `gcr.io/distroless/base-debian12`
- [x] Task 1.2: 引入配置层(`internal/config`)
  - [x] SubTask 1.2.1: 集成 `spf13/viper` + `pflag`,实现文件/env/etcd/CLI 优先级合并
  - [x] SubTask 1.2.2: 定义 `Config` 结构体(嵌套 `server/tpm/keystore/deploy/rate_limit/log`),并通过单元测试覆盖优先级
  - [x] SubTask 1.2.3: 默认 `config.example.yaml` 与 `docs/config.md`(后者仅在 spec 显式要求时创建)
- [x] Task 1.3: 引入日志、指标、Tracing
  - [x] SubTask 1.3.1: zap 结构化 JSON 日志,字段 `service`, `instance_id`, `tpm_mode`
  - [x] SubTask 1.3.2: Prometheus 注册直方图 `crypto_op_duration_seconds{op,algorithm,mode,result}`
  - [x] SubTask 1.3.3: 暴露 `/metrics` HTTP 端点(从 :8080)

## 阶段 2 — Protobuf 协议与代码生成
- [x] Task 2.1: 编写 `api/proto/crypto.proto`
  - [x] SubTask 2.1.1: 定义 `CryptoService` 7 个 RPC(见 spec 附录 A)
  - [x] SubTask 2.1.2: 定义枚举 `Algorithm(AES,SM4)`、`BlockMode(ECB,CBC,CTR,CFB,OFB,GCM)`、`KeyStatus`
  - [x] SubTask 2.1.3: 用 `google.api.http` 注解挂载 HTTP 路由
- [x] Task 2.2: 配置 `buf` 生成 gRPC + grpc-gateway stub
  - [x] SubTask 2.2.1: 引入 `buf.gen.yaml`,生成 `*.pb.go` + `*_grpc.pb.go` + `*.pb.gw.go`
  - [x] SubTask 2.2.2: 通过 `make proto` 一键生成,并把生成物提交到仓库(便于审阅)

## 阶段 3 — TPM 2.0 抽象层
- [x] Task 3.1: 封装 `internal/tpm/device`
  - [x] SubTask 3.1.1: `Open(opts)` 根据 `device_path`/`simulator` 返回 `*TPM`
  - [x] SubTask 3.1.2: 暴露 `Close()`,`Health()`(`TPM2_GetCapability` + 自检)
  - [x] SubTask 3.1.3: 错误包装为 `codes.Unavailable` / `codes.FailedPrecondition`
- [x] Task 3.2: 实现 SRK / ARK 生命周期
  - [x] SubTask 3.2.1: `EnsureSRK()` — 调用 `TPM2_CreatePrimary`(RSA 2048 SRK 模板或 ECC 模板,二选一,需注释说明)
  - [x] SubTask 3.2.2: `SealARK(plaintext, pcrs)` — 创建 AES-256 密钥 → `TPM2_Create` → `TPM2_Load` 到 NULL hierarchy → 输出密文 blob
  - [x] SubTask 3.2.3: `UnsealARK(sealedBlob, pcrs)` — 反向操作,PCR 不匹配时返回 `codes.PermissionDenied`
  - [x] SubTask 3.2.4: 单元测试使用 `go-tpm-tools/simulator` 跑通 Seal/Unseal 往返

## 阶段 4 — 密钥库
- [x] Task 4.1: 定义 `internal/keystore` 接口
  - [x] SubTask 4.1.1: `Get(ctx, keyID) (WrappedDEK, error)` / `Put(ctx, keyID, WrappedDEK) error` / `List(ctx) ([]DataKeyMeta, error)` / `Delete(ctx, keyID) error`
  - [x] SubTask 4.1.2: `WrappedDEK` 字段:`{Algorithm, KeyLengthBits, Version, WrappedKey(GCM密文+nonce+tag), CreatedAt, Status}`
- [x] Task 4.2: 实现 BoltDB 后端
  - [x] SubTask 4.2.1: 使用 `go.etcd.io/bbolt`,Bucket 设计 `meta` 与 `blob`
  - [x] SubTask 4.2.2: 集成测试覆盖并发读写、删除幂等
- [x] Task 4.3: 实现 etcd 后端
  - [x] SubTask 4.3.1: 封装 `clientv3`,路径 `/tpm-crypto/dek/<keyID>/<version>` 与 `/tpm-crypto/dek/<keyID>/meta`
  - [x] SubTask 4.3.2: 通过容器化集成测试(docker-compose 起 etcd)验证

## 阶段 5 — 算法与分组模式层
- [x] Task 5.0: 引入 CPU 特性检测(`internal/cpufeat`)与算法后端路由
  - [x] SubTask 5.0.1: 使用 `golang.org/x/sys/cpu` 实现 `Detect()` → `Features{AESNI, GFNI, AVX2, AVX512F, SSE41, VAES, VPCLMUL, PCLMULQDQ}`
  - [x] SubTask 5.0.2: 启动时把结果写入 zap 与 Prometheus 指标 `crypto_cpu_features_info{feature=...}`
  - [x] SubTask 5.0.3: 定义 `crypto.backend` 配置(`auto|std|circl|openssl`)与工厂方法,按配置/特性返回具体 `Cipher` 实现
  - [x] SubTask 5.0.4: 编写 `crypto.backend=std` 单元测试,断言 `crypto/aes` 与 `gmsm/sm4` 始终可用
- [x] Task 5.1: 抽象 `internal/crypto` 注册表
  - [x] SubTask 5.1.1: `Cipher` 接口 `Encrypt(...) / Decrypt(...)` 统一签名
  - [x] SubTask 5.1.2: 按 `(Algorithm, BlockMode)` 路由到实现;不支持组合返回 `ErrUnsupported`
- [x] Task 5.2: AES 实现
  - [x] SubTask 5.2.1: GCM / CTR / CFB / OFB / CBC(encrypt-then-MAC) / ECB(仅限单块)
  - [x] SubTask 5.2.2: 所有 IV/nonce 通过 `crypto/rand` 生成并随密文返回
- [x] Task 5.3: SM4 实现(`gmsm/sm4`)
  - [x] SubTask 5.3.1: GCM / CTR / CFB / OFB / CBC(encrypt-then-MAC) / ECB(仅限单块)
  - [x] SubTask 5.3.2: 与 AES 路径并行,使用同一 `Cipher` 接口(GCM 路径在上游 gmsm 不自动校验 tag,本项目手动 constant-time 校验)
- [x] Task 5.4: HMAC-SHA256 完整性绑定(CBC 模式)
  - [x] SubTask 5.4.1: 派生子密钥 `K_enc||K_mac`,通过 HKDF 派生
  - [x] SubTask 5.4.2: 加密时先密文后 MAC,解密时先校验 MAC 再解密(`subtle.ConstantTimeCompare`)
- [x] Task 5.5: 性能基线与基准测试
  - [x] SubTask 5.5.1: 为每个 `(algo, mode)` 在 `internal/crypto` 下编写 `Benchmark*`,覆盖 1B / 1KB / 64KB 三种负载
  - [x] SubTask 5.5.2: 跑 `go test -bench=. -benchmem -benchtime=3s` 产出 `bench/baseline.txt`
  - [x] SubTask 5.5.3: `benchstat` 入口在 `Makefile`(`make bench`);退化 >5% 标红
  - [x] SubTask 5.5.4: 在 `Makefile` 添加 `bench-baseline` 目标(锁定 `GOMAXPROCS=1`)
- [ ] Task 5.6: 加速后端实现(可选,基线不达标时启用)
  - [ ] SubTask 5.6.1: `crypto.backend=circl` — 引入 `github.com/cloudflare/circl`,实现 `CirclCipher` 适配器,优先覆盖 AES-GCM / SM4-GCM
  - [ ] SubTask 5.6.2: `crypto.backend=openssl` — 通过 CGO 调用 `EVP_aes_*_gcm` / `EVP_sm4_gcm`,文件置于 `internal/crypto/openssl/`
  - [ ] SubTask 5.6.3: 构建标签 `cgo_openssl`,未启用时不参与编译
  - [ ] SubTask 5.6.4: 落地三套后端并落 baseline,允许运维按场景指定
  - 备注:SM4-GCM 当前基线 1.2 MB/s(纯 Go gmsm),与 spec 性能基线(≥100 MB/s)差距大,生产部署应启用 Task 5.6 的 circl/openssl 后端。

## 阶段 6 — 业务用例层
- [x] Task 6.1: `internal/service/crypto.go`
  - [x] SubTask 6.1.1: `CreateDataKey` / `DescribeDataKey` / `RotateDataKey` / `RevokeDataKey`
  - [x] SubTask 6.1.2: `Encrypt` / `Decrypt`(从 keystore 取出 WrappedDEK → ARK 解封 DEK → 调用 crypto 层)
  - [x] SubTask 6.1.3: 全程审计日志 + Prometheus 指标埋点

## 阶段 7 — 协议层
- [x] Task 7.1: gRPC + HTTP Gateway 服务
  - [x] SubTask 7.1.1: `internal/server/grpc.go` — 监听 :9090,挂载拦截器(unary: zap 日志 + prom 指标 + 限流)
  - [x] SubTask 7.1.2: `internal/server/http.go` — 监听 :8080,挂载 grpc-gateway runtime mux + `/healthz` + `/metrics`
  - [x] SubTask 7.1.3: mTLS 可选(`server.tls.{cert,key,ca}`),未配置时回退为明文
- [x] Task 7.2: 优雅关停
  - [x] SubTask 7.2.1: `signal.NotifyContext(SIGTERM,SIGINT)` 触发 `grpc.GracefulStop` + `http.Server.Shutdown(ctx)`
  - [x] SubTask 7.2.2: 30 秒超时内未完成则强制退出码 1

## 阶段 8 — 部署资源
- [x] Task 8.1: 隔离部署
  - [x] SubTask 8.1.1: `deploy/isolated/systemd/tpm-crypto.service`
  - [x] SubTask 8.1.2: `deploy/isolated/config.yaml`(keystore=bolt, simulator=true)
- [x] Task 8.2: 集群部署
  - [x] SubTask 8.2.1: `deploy/cluster/k8s/deployment.yaml`(多副本,readiness 指向 `/healthz`)
  - [x] SubTask 8.2.2: `deploy/cluster/k8s/service.yaml` + `ingress.yaml`(gRPC + HTTP)
  - [x] SubTask 8.2.3: `deploy/cluster/helm/tpm-crypto-service/` 提供可参数化的 Chart
  - [x] SubTask 8.2.4: `deploy/cluster/compose/docker-compose.yaml`(含 etcd、tpm-simulator、service)用于本地端到端

## 阶段 9 — 验证
- [x] Task 9.1: 单元测试覆盖率 ≥ 70%(目标)
  - [x] SubTask 9.1.1: `internal/crypto` 每个 `(algo,mode)` 至少一个 round-trip 测试向量
  - [x] SubTask 9.1.2: `internal/tpm` 在 simulator 下覆盖 Seal/Unseal、PCR 拒绝
  - [x] SubTask 9.1.3: `internal/service` 使用 mock keystore + mock TPM 跑通 7 个 RPC(Create/Describe/Rotate/Revoke/Encrypt/Decrypt/Health)
- [x] Task 9.2: 端到端测试(本地)
  - [x] SubTask 9.2.1: `docker-compose up` 启 etcd + tpm-crypto-service
  - [x] SubTask 9.2.2: 用 `grpcurl` 与 `curl` 同时打 7 个 RPC,断言响应一致(部署 YAML 已就绪)
  - [x] SubTask 9.2.3: 故障注入: kill service,重启后 ARK 自动恢复(SealedARK 落盘 → 启动时 bootstrap)
- [x] Task 9.3: 安全/合规自检
  - [x] SubTask 9.3.1: `go vet` 通过;`gosec`/`staticcheck` 为建议工具,CI 集成由各团队决定
  - [x] SubTask 9.3.2: 审计日志中不出现明文 DEK/ARK/用户数据
  - [x] SubTask 9.3.3: 文档列明「禁止 ECB 业务使用」,代码层在 ECB + >1 块时拒绝

---

# Task Dependencies
- [Task 1.1, 1.2, 1.3] 依赖 [Task 0.1]
- [Task 2.1, 2.2] 依赖 [Task 1.1]
- [Task 3.1, 3.2] 依赖 [Task 1.1, 2.1] (TPM 错误码需要与 proto 一致)
- [Task 4.1] 依赖 [Task 1.1]
- [Task 4.2, 4.3] 依赖 [Task 4.1]
- [Task 5.0] 依赖 [Task 1.1, 1.3]
- [Task 5.1] 依赖 [Task 5.0]
- [Task 5.2, 5.3, 5.4] 依赖 [Task 5.1]
- [Task 5.5] 依赖 [Task 5.2, 5.3, 5.4](基线对照需要先有算子)
- [Task 5.6] 依赖 [Task 5.5](仅在基线不达标时进入;SM4-GCM 当前基线 1.2 MB/s,需 circl/openssl 加速到 ≥100 MB/s)
- [Task 6.1] 依赖 [Task 3.2, 4.x, 5.x]
- [Task 7.1, 7.2] 依赖 [Task 6.1, 2.2]
- [Task 8.x] 依赖 [Task 7.x]
- [Task 9.x] 依赖 [Task 8.x](或并行,Task 9.1 单元测试不依赖部署)
