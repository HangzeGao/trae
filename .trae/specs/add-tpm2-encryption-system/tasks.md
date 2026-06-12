# Tasks

- [ ] Task 1: TPM 2.0 根密钥管理模块
  - [ ] SubTask 1.1: 创建 internal/tpm/context.go — TPM 上下文初始化（连接 TPM 设备、创建 ESAPI 上下文、错误处理）
  - [ ] SubTask 1.2: 创建 internal/tpm/rootkey.go — 根密钥创建（创建 SRK）和加载（加载已有 SRK）功能
  - [ ] SubTask 1.3: 创建 internal/tpm/datakey.go — 使用根密钥在 TPM 内加密/解密数据密钥

- [ ] Task 2: 密钥服务（KeyServer）HTTP 服务层
  - [ ] SubTask 2.1: 创建 internal/keyserver/server.go — HTTP 服务框架（路由、中间件、TLS 配置）
  - [ ] SubTask 2.2: 创建 internal/keyserver/handlers.go — 实现密钥生成/解密/轮换/算法查询 API handler
  - [ ] SubTask 2.3: 创建 internal/keyserver/service.go — 业务逻辑层，编排 TPM 操作和密钥管理

- [ ] Task 3: 加解密客户端
  - [ ] SubTask 3.1: 创建 internal/client/client.go — EncryptionClient 实现，支持配置密钥服务地址（多地址故障转移）
  - [ ] SubTask 3.2: 实现 client.Encrypt — 调用密钥服务解密数据密钥 + 本地 AES/SM4 加密
  - [ ] SubTask 3.3: 实现 client.Decrypt — 调用密钥服务解密数据密钥 + 本地 AES/SM4 解密
  - [ ] SubTask 3.4: 实现 client.GenerateKey / RotateKey — 代理密钥服务的密钥操作

- [ ] Task 4: 补充 API 模型（DecryptKeyRequest/Response）
  - [ ] SubTask 4.1: 在 internal/model/api.go 中补充密钥解密请求/响应模型

- [ ] Task 5: 启动入口
  - [ ] SubTask 5.1: 创建 cmd/keyserver/main.go — 密钥服务启动入口，读取配置、初始化 TPM、启动 HTTP 服务
  - [ ] SubTask 5.2: 创建 cmd/client/main.go — 客户端示例入口，演示加解密流程

- [ ] Task 6: 配置管理
  - [ ] SubTask 6.1: 创建 internal/config/config.go — 服务配置结构（监听地址、TLS 证书路径、TPM 设备路径等）

- [ ] Task 7: 集成测试
  - [ ] SubTask 7.1: 编写 TPM 模拟器下的端到端测试（密钥生成→加密→解密→轮换）
  - [ ] SubTask 7.2: 编写多节点共享密钥服务的集成测试
  - [ ] SubTask 7.3: 编写客户端故障转移测试

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on 无（可与 Task 1 并行）
- [Task 5] depends on [Task 2, Task 6]
- [Task 6] depends on 无（可与 Task 1 并行）
- [Task 7] depends on [Task 5]
