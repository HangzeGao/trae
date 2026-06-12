# Tasks

- [ ] Task 1: 项目基础结构搭建
  - [ ] SubTask 1.1: 初始化 Go 模块（go mod init），引入 go-tpm 依赖
  - [ ] SubTask 1.2: 创建项目目录结构（cmd/、internal/tpm/、internal/crypto/、internal/api/、internal/model/）
  - [ ] SubTask 1.3: 定义基础类型和错误码（model 包）

- [ ] Task 2: TPM 2.0 根密钥管理模块（internal/tpm/）
  - [ ] SubTask 2.1: 实现 TPM 上下文初始化（打开 /dev/tpmrm0 或 swtpm 连接）
  - [ ] SubTask 2.2: 实现根密钥创建（CreatePrimary，创建 SRK）
  - [ ] SubTask 2.3: 实现根密钥加载（从持久化句柄加载已有 SRK）
  - [ ] SubTask 2.4: 实现 TPM 不可用时的错误处理

- [ ] Task 3: 数据密钥管理模块（internal/tpm/）
  - [ ] SubTask 3.1: 实现数据密钥生成（TPM Create 生成对称密钥，返回公私钥 blob）
  - [ ] SubTask 3.2: 实现使用根密钥加密数据密钥（TPM 内部密钥封装）
  - [ ] SubTask 3.3: 实现使用根密钥解密数据密钥（TPM Load + 解封）
  - [ ] SubTask 3.4: 实现加密数据密钥的持久化存储接口（文件存储）

- [ ] Task 4: 数据加解密模块（internal/crypto/）
  - [ ] SubTask 4.1: 实现 AES-256-GCM 加密函数
  - [ ] SubTask 4.2: 实现 AES-256-GCM 解密函数
  - [ ] SubTask 4.3: 实现密文格式封装（包含 IV、认证标签、密文的序列化）

- [ ] Task 5: REST API 服务（internal/api/）
  - [ ] SubTask 5.1: 实现 HTTP 服务启动和路由注册
  - [ ] SubTask 5.2: 实现 POST /api/v1/keys/generate 接口
  - [ ] SubTask 5.3: 实现 POST /api/v1/encrypt 接口
  - [ ] SubTask 5.4: 实现 POST /api/v1/decrypt 接口
  - [ ] SubTask 5.5: 实现 POST /api/v1/keys/rotate 接口

- [ ] Task 6: 主程序入口和配置（cmd/）
  - [ ] SubTask 6.1: 实现 main.go 入口，初始化 TPM 连接和 HTTP 服务
  - [ ] SubTask 6.2: 实现配置管理（TPM 设备路径、监听端口等）

- [ ] Task 7: 集成测试与验证
  - [ ] SubTask 7.1: 编写根密钥创建/加载的单元测试
  - [ ] SubTask 7.2: 编写数据密钥生成/加密/解密的单元测试
  - [ ] SubTask 7.3: 编写端到端加解密流程的集成测试（使用 swtpm 模拟器）
  - [ ] SubTask 7.4: 编写密钥轮换流程的集成测试

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 1]
- [Task 5] depends on [Task 3, Task 4]
- [Task 6] depends on [Task 5]
- [Task 7] depends on [Task 6]
