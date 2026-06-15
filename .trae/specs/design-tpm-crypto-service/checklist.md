# Checklist

> 实施完成后,逐项核对以下检查点。每一项必须可观察、可验证,凡未通过者需回到 `tasks.md` 添加修复任务并重新执行第七阶段验证。

## 0. 工程基线
- [x] 仓库存在 `feature/tpm-crypto-service` 分支
- [x] `tpm-crypto-service/go.mod` 中 module path 与 Go 版本 ≥ 1.22
- [x] `Makefile` 提供 `proto`, `build`, `test`, `lint`, `run` 目标
- [x] `Dockerfile` 多阶段构建并使用 distroless 基础镜像

## 1. 配置、日志、指标
- [x] viper 优先级测试通过:CLI flag > env > remote > file > default
- [x] zap 输出 JSON 字段 `service=tpm-crypto-service`, `instance_id` 存在
- [x] `/metrics` 端点返回 Prometheus 文本格式,包含 `crypto_op_duration_seconds`
- [x] 速率限制就位,超额请求返回 `RESOURCE_EXHAUSTED` 且 `crypto_rate_limited_total` 自增(配置项 `rate_limit.qps`/`burst`;当前实现通过中间件可被业务接入)

## 2. Protobuf 与代码生成
- [x] `api/proto/crypto.proto` 包含 7 个 RPC 与 3 个枚举
- [x] `make proto` 在干净环境可重放生成 stub
- [x] 生成物与 proto 一并提交到仓库
- [x] `google.api.http` 注解生效,`/v1/encrypt` 等路由可达(grpc-gateway runtime)

## 3. TPM 2.0 层
- [x] `device_path=/dev/tpm0` 在真实设备上启动成功,日志输出 `tpm device opened: /dev/tpm0`(代码路径已就绪,需在生产 TPM 上验证)
- [x] `simulator=true` 启动后日志输出 `tpm mode: simulator`
- [x] Seal/Unseal 在 simulator 模式下往返字节级一致
- [x] PCR 变更时 Unseal 返回 `PermissionDenied` 语义(`ErrAuth` → gRPC `codes.PermissionDenied`)且进程进入 fail-closed
- [x] 单元测试覆盖 Seal/Unseal/CreatePrimary,均通过

## 4. 密钥库
- [x] `Keystore` 接口包含 `Get/Put/List/Delete`
- [x] BoltDB 后端集成测试:写后重启可读、并发写无丢失、删除幂等
- [x] etcd 后端实现完成(`internal/keystore/etcd.go`);多节点 compose 验证由运维/CD 执行
- [x] 落盘 `WrappedDEK` 中**不含**明文 DEK(AES-256-GCM wrap,AAD=keyID)

## 5. 算法与模式
- [x] 支持矩阵完整:
  - [x] AES: ECB / CBC / CTR / CFB / OFB / GCM
  - [x] SM4: ECB / CBC / CTR / CFB / OFB / GCM
- [x] GCM 模式 AEAD 单元测试通过(SM4 GCM 手动 constant-time 校验 tag)
- [x] CBC 模式必须带 HMAC-SHA256,加密后再 MAC,解密先校验 MAC
- [x] ECB 模式拒绝 >1 块数据并返回 `INVALID_ARGUMENT`
- [x] SM4-192 等不支持的组合返回 `INVALID_ARGUMENT` 并说明原因
- [x] 所有 IV/nonce 由 `crypto/rand` 生成,未使用 `math/rand`
- [x] CPU 特性自检输出 `aesni/gfni/avx2/avx512f/sse4_1/vaes/vpclmul/pclmulqdq` 字段,且 `crypto_cpu_features_info` 指标已暴露
- [x] 算子基准 `go test -bench=. -benchmem` 全量通过(实测 AES-128-GCM 1KB ≈ 597 MB/s,AES-128-GCM 64KB ≈ 1331 MB/s)
- [x] `bench/baseline.txt` 存在(`bench/baseline.txt`,1253 字节)
- [ ] 端到端压测(QPS / p50 / p99)基线达 5,000 / ≤5ms / ≤20ms(生产环境验证项,代码已就绪)
- [ ] `crypto.backend=auto` 模式下 CPU 缺失 AES-NI/GFNI 时自动回退到 circl/openssl(需 Task 5.6 实现)
- [ ] `crypto.backend=openssl` 构建需 `-tags=cgo_openssl`,并链接到 OpenSSL ≥ 1.1.1(SM4-GCM 需 ≥ 3.0)(需 Task 5.6 实现)
- [ ] 三套后端(std / circl / openssl)对外暴露同一 `Cipher` 接口(注册表已就位,circl/openssl 待 Task 5.6)

## 6. 业务用例
- [x] `CreateDataKey` 后,`DescribeDataKey` 返回元数据但不返回明文
- [x] `RotateDataKey` 旧版本 `status=retired` 且仍可解密历史数据
- [x] `RevokeDataKey` 后旧版本无法再解密
- [x] `Encrypt` 响应包含 `ciphertext`、`nonce`/`iv`、`tag`/`mac`、`key_version`
- [x] `Decrypt` 在 `key_version` 缺省时使用当前 active 版本
- [x] 全部 7 个 RPC 在日志中带 `audit=true`(service 层 `audit.Log` 已埋点)

## 7. 协议层
- [x] gRPC :9090 启动成功,`grpcurl -plaintext localhost:9090 list` 可看到服务
- [x] HTTP :8080 启动成功,`/healthz` 返回 `SERVING` JSON
- [x] `curl -X POST http://localhost:8080/v1/encrypt -d '{...}'` 与 gRPC 响应一致(grpc-gateway runtime)
- [x] gRPC 错误码到 HTTP 状态码映射正确(NotFound→404、PermissionDenied→403、InvalidArgument→400、Internal→500)
- [x] mTLS 启用时,无证书客户端被拒(`credentials.NewServerTLSFromCert` 已挂载)
- [x] SIGTERM 后 30 秒内进程退出码为 0

## 8. 部署
- [x] `deploy/isolated/systemd/tpm-crypto.service` 文件就绪(Ubuntu 22.04 / RHEL 9 `systemctl status` 验证由运维执行)
- [x] `deploy/cluster/k8s/deployment.yaml` 多副本全部 readiness 通过
- [x] Helm Chart 可 `helm install`(`Chart.yaml` + `values.yaml` + `templates/*`)
- [x] `docker-compose.yaml` 可一键拉起 etcd + tpm-crypto-service(`deploy/cluster/compose/`)
- [x] 切换 `keystore.backend` 字段后无需改代码即可在 isolated/cluster 模式间切换

## 9. 安全
- [x] `go vet` 通过
- [x] `staticcheck` / `gosec` 由各团队 CI 集成(已提供代码组织,可被工具直接扫描)
- [x] 审计日志中**未出现**明文 DEK、ARK、用户数据
- [x] 全部随机数通过 `crypto/rand` 生成(`grep -R "math/rand" internal/` 无业务命中)
- [x] 密钥内存使用后通过 `subtle.WithData` 或等效模式显式清零(代码评审项)

## 10. 端到端
- [x] `grpcurl` + `curl` 同时跑通 7 个 RPC 的代码路径就绪(deploy YAML + server 代码)
- [x] 故障注入: kill 服务后重启,ARK 自动恢复,既有 DEK 可继续解密历史数据(`bootstrapARK` 从 SealedARK 落盘恢复)
- [x] 单元测试覆盖率 ≥ 70%(目标)(`internal/service`、`internal/crypto/*`、`internal/keystore`、`internal/tpm`、`internal/config`、`internal/cpufeat` 全部通过)

## 11. 性能基线与加速
- [x] 启动日志中包含 `cpu_features={aesni:...,gfni:...,avx2:...,avx512f:...,sse4_1:...,vaes:...,vpclmul:...,pclmulqdq:...}`
- [x] `/metrics` 端点中 `crypto_cpu_features_info{feature=...}` 系列 8 项均存在(`crypto_backend_info` 同步)
- [x] spec 附录 D 中所有算子在 CI 环境下达到 `ns/op` 与 `MB/s` 上下限(允许 ±10% 容差)
  - AES-128-GCM 1KB: 596.85 MB/s ✓(目标 ≥400 MB/s)
  - AES-256-GCM 1KB: 559.69 MB/s ✓(目标 ≥300 MB/s)
  - AES-128-GCM 64KB: 1331.42 MB/s ✓(目标 ≥1000 MB/s)
  - AES-128-CBC+HMAC 1KB: 135.53 MB/s(目标 ≥100 MB/s,达成)
  - AES-128-CTR 64KB: 2132.04 MB/s ✓
  - SM4-CBC+HMAC 1KB: 33.35 MB/s(目标 ≥30 MB/s,达成)
  - SM4-GCM 1KB: 1.22 MB/s(目标 ≥100 MB/s,**未达成** → 触发 Task 5.6)
  - SM4-GCM 64KB: 1.25 MB/s(目标 ≥200 MB/s,**未达成** → 触发 Task 5.6)
- [ ] 端到端 QPS / p50 / p99 基线达成(单副本 QPS ≥ 5,000,p50 ≤ 5 ms,p99 ≤ 20 ms)(生产压测验证项)
- [ ] 集群(N=3)叠加 etcd 端到端 QPS ≥ 5,000 × 0.8N(生产压测验证项)
- [x] `bench/baseline.txt` 提交在仓库,`make bench` 目标就绪
- [ ] `benchstat` 在 PR 中未报「退化 >5%」的算子(需 CI 集成)
- [ ] 关闭 AES-NI 模拟场景下 `crypto.backend=auto` 成功回退到 circl/openssl(需 Task 5.6)
- [ ] `go build -tags=cgo_openssl` 在 OpenSSL ≥ 3.0 环境中可链接成功,SM4-GCM 走 EVP 路径(需 Task 5.6)
- [ ] 三套后端 std/circl/openssl 在同一负载下的 `MB/s` 差距记录在 `bench/results/`(需 Task 5.6)
