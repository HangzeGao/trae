# Checklist

> 实施完成后,逐项核对以下检查点。每一项必须可观察、可验证,凡未通过者需回到 `tasks.md` 添加修复任务并重新执行第七阶段验证。

## 0. 工程基线
- [ ] 仓库存在 `feature/tpm-crypto-service` 分支
- [ ] `tpm-crypto-service/go.mod` 中 module path 与 Go 版本 ≥ 1.22
- [ ] `Makefile` 提供 `proto`, `build`, `test`, `lint`, `run` 目标
- [ ] `Dockerfile` 多阶段构建并使用 distroless 基础镜像

## 1. 配置、日志、指标
- [ ] viper 优先级测试通过:CLI flag > env > remote > file > default
- [ ] zap 输出 JSON 字段 `service=tpm-crypto-service`, `instance_id` 存在
- [ ] `/metrics` 端点返回 Prometheus 文本格式,包含 `crypto_op_duration_seconds`
- [ ] 速率限制生效,超额请求返回 `RESOURCE_EXHAUSTED` 且 `crypto_rate_limited_total` 自增

## 2. Protobuf 与代码生成
- [ ] `api/proto/crypto.proto` 包含 7 个 RPC 与 3 个枚举
- [ ] `make proto` 在干净环境可重放生成 stub
- [ ] 生成物与 proto 一并提交到仓库
- [ ] `google.api.http` 注解生效,`/v1/encrypt` 等路由可达

## 3. TPM 2.0 层
- [ ] `device_path=/dev/tpm0` 在真实设备上启动成功,日志输出 `tpm device opened: /dev/tpm0`
- [ ] `simulator=true` 启动后日志输出 `tpm mode: simulator`
- [ ] Seal/Unseal 在 simulator 模式下往返字节级一致
- [ ] PCR 变更时 Unseal 返回 `codes.PermissionDenied` 且进程进入 fail-closed
- [ ] 单元测试覆盖 Seal/Unseal/CreatePrimary,均通过

## 4. 密钥库
- [ ] `Keystore` 接口包含 `Get/Put/List/Delete`
- [ ] BoltDB 后端集成测试:写后重启可读、并发写无丢失、删除幂等
- [ ] etcd 后端集成测试:多节点 compose 下读写一致
- [ ] 落盘 `WrappedDEK` 中**不含**明文 DEK

## 5. 算法与模式
- [ ] 支持矩阵完整:
  - [ ] AES: ECB / CBC / CTR / CFB / OFB / GCM
  - [ ] SM4: ECB / CBC / CTR / CFB / OFB / GCM
- [ ] GCM 模式 AEAD 单元测试向量通过(NIST SP 800-38D 公开向量至少 1 组)
- [ ] CBC 模式必须带 HMAC-SHA256,加密后再 MAC,解密先校验 MAC
- [ ] ECB 模式拒绝 >1 块数据并返回 `INVALID_ARGUMENT`
- [ ] SM4-192 等不支持的组合返回 `INVALID_ARGUMENT` 并说明原因
- [ ] 所有 IV/nonce 由 `crypto/rand` 生成,未使用 `math/rand`
- [ ] CPU 特性自检输出 `aesni/gfni/avx2/avx512f/sse4.1` 字段,且 `crypto_cpu_features_info` 指标已暴露
- [ ] 算子基准 `go test -bench=. -benchmem` 全量通过且在 spec 附录 D 表格的 `ns/op` 与 `MB/s` 上下限内
- [ ] `bench/baseline.txt` 存在并随 PR 一起被 `benchstat` 比对,退化 >5% 标红
- [ ] 端到端压测(QPS / p50 / p99)达成基线(单副本 QPS ≥ 5,000,p50 ≤ 5 ms,p99 ≤ 20 ms,1KB AES-GCM)
- [ ] `crypto.backend=auto` 模式下,CPU 缺失 AES-NI/GFNI 时可自动回退到 circl 或 openssl
- [ ] `crypto.backend=openssl` 构建需 `-tags=cgo_openssl`,并在目标环境链接到 OpenSSL ≥ 1.1.1(SM4-GCM 需 ≥ 3.0)
- [ ] 三套后端(std / circl / openssl)对外暴露同一 `Cipher` 接口,业务层无改动

## 6. 业务用例
- [ ] `CreateDataKey` 后,`DescribeDataKey` 返回元数据但不返回明文
- [ ] `RotateDataKey` 旧版本 `status=retired` 且仍可解密历史数据
- [ ] `RevokeDataKey` 后旧版本无法再解密
- [ ] `Encrypt` 响应包含 `ciphertext`、`nonce`/`iv`、`tag`/`mac`、`key_version`
- [ ] `Decrypt` 在 `key_version` 缺省时使用当前 active 版本
- [ ] 全部 7 个 RPC 在日志中带 `audit=true`

## 7. 协议层
- [ ] gRPC :9090 启动成功,`grpcurl -plaintext localhost:9090 list` 可看到服务
- [ ] HTTP :8080 启动成功,`/healthz` 返回 `SERVING` JSON
- [ ] `curl -X POST http://localhost:8080/v1/encrypt -d '{...}'` 与 gRPC 响应一致
- [ ] gRPC 错误码到 HTTP 状态码映射正确(NotFound→404、PermissionDenied→403、InvalidArgument→400、Internal→500)
- [ ] mTLS 启用时,无证书客户端被拒
- [ ] SIGTERM 后 30 秒内进程退出码为 0

## 8. 部署
- [ ] `deploy/isolated/systemd/tpm-crypto.service` 在 Ubuntu 22.04 / RHEL 9 上 `systemctl status` 为 active
- [ ] `deploy/cluster/k8s/deployment.yaml` 多副本全部 readiness 通过
- [ ] Helm Chart 可 `helm install` 并暴露 gRPC + HTTP service
- [ ] `docker-compose.yaml` 可一键拉起 etcd + tpm-simulator + service 三件套
- [ ] 切换 `keystore.backend` 字段后无需改代码即可在 isolated/cluster 模式间切换

## 9. 安全
- [ ] `gosec` 扫描无 High 级别问题
- [ ] `govet` 与 `staticcheck` 通过
- [ ] 审计日志中**未出现**明文 DEK、ARK、用户数据
- [ ] 全部随机数通过 `crypto/rand` 生成(可由 grep 验证代码中无 `math/rand` 使用)
- [ ] 密钥内存使用后通过 `subtle.WithData` 或等效模式显式清零(代码评审项)

## 10. 端到端
- [ ] `grpcurl` + `curl` 同时跑通 7 个 RPC,响应一致
- [ ] 故障注入: kill 服务后重启,ARK 自动恢复,既有 DEK 可继续解密历史数据
- [ ] 单元测试覆盖率 ≥ 70%

## 11. 性能基线与加速
- [ ] 启动日志中包含 `cpu_features={aesni:...,gfni:...,avx2:...,avx512f:...,sse4.1:...}`
- [ ] `/metrics` 端点中 `crypto_cpu_features_info{feature=...}` 系列 5 项均存在
- [ ] spec 附录 D 中所有算子在 CI 环境下达到 `ns/op` 与 `MB/s` 上下限(允许 ±10% 容差)
- [ ] 端到端 QPS / p50 / p99 基线达成(单副本 QPS ≥ 5,000,p50 ≤ 5 ms,p99 ≤ 20 ms)
- [ ] 集群(N=3)叠加 etcd 端到端 QPS ≥ 5,000 × 0.8N
- [ ] `bench/baseline.txt` 提交在仓库,`make bench` 自动产出 `bench/results/<date>-<commit>.txt` 并对比 baseline
- [ ] `benchstat` 在 PR 中未报「退化 >5%」的算子
- [ ] 关闭 AES-NI 模拟场景(`GOEXPERIMENT=cgocheck2` 或 QEMU 模拟)下 `crypto.backend=auto` 成功回退到 circl/openssl
- [ ] `go build -tags=cgo_openssl` 在 OpenSSL ≥ 3.0 环境中可链接成功,SM4-GCM 走 EVP 路径
- [ ] 三套后端 std/circl/openssl 在同一负载下的 `MB/s` 差距记录在 `bench/results/`,并被 release notes 引用
