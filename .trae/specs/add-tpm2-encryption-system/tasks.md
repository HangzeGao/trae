# Tasks

- [ ] Task 1: internal/model — 补充 DecryptKey 请求/响应类型
  - [ ] SubTask 1.1: 在 [api.go](file:///workspace/internal/model/api.go) 中新增 `DecryptKeyRequest` 结构体（字段 `EncryptedDataKey []byte`）
  - [ ] SubTask 1.2: 在 [api.go](file:///workspace/internal/model/api.go) 中新增 `DecryptKeyResponse` 结构体（字段 `PlaintextDataKey []byte`）
  - [ ] SubTask 1.3: 在 [api.go](file:///workspace/internal/model/api.go) 中新增 `HealthResponse` 结构体（字段 `Status string`, `TPMAvailable bool`）

- [ ] Task 2: go.mod — 添加 TPM 2.0 库依赖
  - [ ] SubTask 2.1: 在 [go.mod](file:///workspace/go.mod) 中添加 `github.com/google/go-tpm v0.9.0`
  - [ ] SubTask 2.2: 运行 `go mod tidy` 生成 go.sum

- [ ] Task 3: internal/tpm — TPM 2.0 驱动层
  - [ ] SubTask 3.1: 创建 `internal/tpm/context.go` — `TPMContext` 结构体
    - 字段：`rwc io.ReadWriteCloser`（TPM 读写通道）、`srkHandle tpm2.Handle`
    - `Open(devicePath string) error` — 调用 `tpm2.OpenTPM(devicePath)` 打开设备，失败返回 `model.ErrTPMNotAvailable`
    - `Close() error` — 关闭 TPM 连接，清理 SRK handle
  - [ ] SubTask 3.2: 创建 `internal/tpm/rootkey.go` — SRK 根密钥管理
    - `CreateOrLoadSRK() (*rsa.PublicKey, tpm2.Handle, error)` — 在 Owner hierarchy 调用 `tpm2.CreatePrimary` 创建 RSA 2048 受限解密密钥，解析公钥返回
    - `GetPublicKey() *rsa.PublicKey` — 返回 SRK 的 RSA 公钥
  - [ ] SubTask 3.3: 创建 `internal/tpm/datakey.go` — 数据密钥加解密
    - `GenerateAndEncryptDataKey(algorithm model.Algorithm, srkPub *rsa.PublicKey) (encryptedKey []byte, plaintextKey []byte, err error)` — 调用 `crypto/rand.Read` 生成对应长度随机密钥，调用 `rsa.EncryptOAEP(crypto.SHA256, rand.Reader, srkPub, plaintextKey, nil)` 加密
    - `DecryptDataKey(srkHandle tpm2.Handle, encryptedDataKey []byte) (plaintextKey []byte, err error)` — 在 TPM 内调用 `tpm2.RSADecrypt`（OAEP with SHA-256, empty label），失败返回 `model.ErrKeyDecryptFailed`
  - [ ] SubTask 3.4: 创建 `internal/tpm/tpm.go` — 顶层 `TPMService` 类型
    - 封装 `TPMContext` + `RootKey`，对外暴露 `GenerateKey(algorithm)`, `DecryptKey(encrypted)`, `RotateKey(oldEncrypted, algorithm)`
    - 初始化方法 `NewTPMService(devicePath string) (*TPMService, error)`

- [ ] Task 4: internal/config — 配置管理
  - [ ] SubTask 4.1: 创建 `internal/config/config.go` — `KeyServerConfig` 和 `ClientConfig`
    - `KeyServerConfig`: `TPMDevicePath` (string, default `/dev/tpm0`), `ListenAddr` (string, default `:8080`), `TLSCertPath` (string), `TLSKeyPath` (string), `ReadTimeout` (int seconds), `WriteTimeout` (int seconds)
    - `ClientConfig`: `KeyServerURLs` ([]string), `TLSCACertPath` (string), `InsecureSkipVerify` (bool)
    - `LoadKeyServerConfig() *KeyServerConfig` — 从环境变量读取（TPM_DEVICE, LISTEN_ADDR, TLS_CERT, TLS_KEY）
    - `LoadClientConfig() *ClientConfig` — 从环境变量读取（KEY_SERVER_URLS 逗号分隔, TLS_CA_CERT, INSECURE_SKIP_VERIFY）

- [ ] Task 5: internal/keyserver — 密钥服务 HTTP 层
  - [ ] SubTask 5.1: 创建 `internal/keyserver/service.go` — 业务逻辑层 `KeyService`
    - 持有 `*tpm.TPMService` 引用
    - `HandleGenerate(req *model.GenerateKeyRequest) (*model.GenerateKeyResponse, error)` — 调用 tpm 层生成并加密密钥
    - `HandleDecrypt(req *model.DecryptKeyRequest) (*model.DecryptKeyResponse, error)` — 调用 tpm 层解密数据密钥
    - `HandleRotate(req *model.RotateKeyRequest) (*model.RotateKeyResponse, error)` — 调用 tpm 层生成新密钥
    - `HandleAlgorithms() *model.AlgorithmsResponse` — 返回所有支持的算法和模式列表
    - `HealthCheck() *model.HealthResponse` — 返回健康状态
  - [ ] SubTask 5.2: 创建 `internal/keyserver/handlers.go` — HTTP handler
    - `writeError(w http.ResponseWriter, code int, err error)` — 统一错误响应
    - `handleGenerateKey(w, r)`, `handleDecryptKey(w, r)`, `handleRotateKey(w, r)`, `handleAlgorithms(w, r)`, `handleHealth(w, r)`
    - 所有 handler 正确处理 JSON 解析、base64 字节、错误码映射
  - [ ] SubTask 5.3: 创建 `internal/keyserver/server.go` — HTTP Server
    - `NewServer(cfg *config.KeyServerConfig, svc *KeyService) *http.Server` — 注册路由（`/api/v1/keys/generate`, `/api/v1/keys/decrypt`, `/api/v1/keys/rotate`, `/api/v1/algorithms`, `/health`）
    - 支持 TLS 和非 TLS 两种启动方式

- [ ] Task 6: internal/client — 加解密客户端
  - [ ] SubTask 6.1: 创建 `internal/client/client.go` — `EncryptionClient`
    - 字段：`keyServerURLs []string`, `httpClient *http.Client`, `keyCache map[string][]byte`（encryptedKey → plaintextKey 缓存）
    - `NewClient(cfg *config.ClientConfig) (*EncryptionClient, error)` — 初始化 HTTP Client（含 TLS 配置）
    - `getPlaintextKey(encryptedDataKey []byte) ([]byte, error)` — 先查缓存，否则请求 `/api/v1/keys/decrypt`，带故障转移
    - `Encrypt(plaintext, encryptedDataKey []byte, algorithm model.Algorithm, mode model.Mode) ([]byte, error)` — 获取明文密钥 → 创建 `crypto.Cipher` → 加密 → 返回 CiphertextFormat 的 JSON 字节，用完清零密钥
    - `Decrypt(ciphertextBytes, encryptedDataKey []byte, algorithm model.Algorithm, mode model.Mode) ([]byte, error)` — 获取明文密钥 → 反序列化 CiphertextFormat → 创建 `crypto.Cipher` → 解密 → 返回明文，用完清零密钥
    - `GenerateKey(algorithm model.Algorithm) (encryptedKey []byte, plaintextKey []byte, err error)` — 代理 `/api/v1/keys/generate`
    - `RotateKey(oldEncryptedKey []byte, algorithm model.Algorithm) (newEncryptedKey []byte, newPlaintextKey []byte, err error)` — 代理 `/api/v1/keys/rotate`

- [ ] Task 7: cmd/keyserver — 服务启动入口
  - [ ] SubTask 7.1: 创建 `cmd/keyserver/main.go` — main 函数
    - 读取 `KeyServerConfig`
    - 创建 `tpm.NewTPMService(devicePath)`
    - 创建 `keyserver.NewServer(...)`
    - 根据 TLS 配置调用 `server.ListenAndServe()` 或 `server.ListenAndServeTLS()`
    - 注册信号处理（SIGINT/SIGTERM）优雅关闭

- [ ] Task 8: cmd/client — 客户端示例
  - [ ] SubTask 8.1: 创建 `cmd/client/main.go` — 演示端到端流程
    - 创建 `client.NewClient(cfg)`
    - 调用 `client.GenerateKey(AES-256)` 生成密钥
    - 调用 `client.Encrypt("测试数据", encryptedKey, AES-256, GCM)` 加密
    - 调用 `client.Decrypt(ciphertext, encryptedKey, AES-256, GCM)` 解密
    - 打印结果验证加解密一致性

- [ ] Task 9: 集成测试
  - [ ] SubTask 9.1: 创建 `internal/tpm/tpm_test.go` — TPM 层单元测试（可用 TPM 模拟器或 mock）
  - [ ] SubTask 9.2: 创建 `internal/client/client_test.go` — 客户端故障转移测试
  - [ ] SubTask 9.3: 创建 `internal/keyserver/integration_test.go` — 端到端加解密流程测试
  - [ ] SubTask 9.4: 创建 `internal/model/api_test.go` — JSON 序列化/反序列化测试

# Task Dependencies

- [Task 1] 独立（仅修改 model 层）
- [Task 2] 独立（仅 go.mod）
- [Task 3] depends on [Task 2]
- [Task 4] 独立
- [Task 5] depends on [Task 1, Task 3, Task 4]
- [Task 6] depends on [Task 1, Task 4]
- [Task 7] depends on [Task 3, Task 4, Task 5]
- [Task 8] depends on [Task 4, Task 6]
- [Task 9] depends on [Task 1, Task 3, Task 5, Task 6]

# 可并行任务

- Task 1、Task 2、Task 4 可并行执行
- Task 3（TPM 层）与 Task 1、Task 4 可并行
- Task 5（KeyServer）与 Task 6（Client）可并行（各自依赖不同路径）
