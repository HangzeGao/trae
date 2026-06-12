# Tasks

- [ ] Task 1: 项目基础结构搭建
  - [ ] SubTask 1.1: 初始化 Go 模块（go mod init），引入 go-tpm 依赖
  - [ ] SubTask 1.2: 创建项目目录结构（cmd/、internal/tpm/、internal/crypto/、internal/api/、internal/model/、internal/config/）
  - [ ] SubTask 1.3: 定义基础类型、错误码和算法/模式枚举（model 包）

- [ ] Task 2: TPM 2.0 根密钥管理模块（internal/tpm/）
  - [ ] SubTask 2.1: 实现 TPM 上下文初始化（打开 /dev/tpmrm0 或 swtpm 连接）
  - [ ] SubTask 2.2: 实现根密钥创建（CreatePrimary，创建 SRK）
  - [ ] SubTask 2.3: 实现根密钥加载（从持久化句柄加载已有 SRK）
  - [ ] SubTask 2.4: 实现 TPM 不可用时的错误处理

- [ ] Task 3: 数据密钥管理模块（internal/tpm/）
  - [ ] SubTask 3.1: 实现数据密钥生成（支持 AES-128/192/256 和 SM4-128 密钥长度）
  - [ ] SubTask 3.2: 实现使用根密钥加密数据密钥（TPM 内部密钥封装）
  - [ ] SubTask 3.3: 实现使用根密钥解密数据密钥（TPM Load + 解封）
  - [ ] SubTask 3.4: 实现加密数据密钥的持久化存储接口（文件存储）

- [ ] Task 4: 多算法多模式加解密模块（internal/crypto/）
  - [ ] SubTask 4.1: 实现 AES 加解密引擎（支持 GCM/CBC/CTR/CFB/OFB 模式，128/192/256 位密钥）
  - [ ] SubTask 4.2: 实现 SM4 加解密引擎（支持 GCM/CBC/CTR/CFB/OFB 模式，128 位密钥）
  - [ ] SubTask 4.3: 实现统一的 Cipher 接口，根据算法+模式自动选择引擎
  - [ ] SubTask 4.4: 实现密文格式封装（包含 IV/Nonce、认证标签、算法标识、模式标识、密文）
  - [ ] SubTask 4.5: 实现算法和模式的校验逻辑（密钥长度匹配、模式合法性检查）

- [ ] Task 5: REST API 服务（internal/api/）
  - [ ] SubTask 5.1: 实现 HTTP 服务启动和路由注册
  - [ ] SubTask 5.2: 实现 POST /api/v1/keys/generate 接口（支持指定算法）
  - [ ] SubTask 5.3: 实现 POST /api/v1/encrypt 接口（支持指定算法和模式）
  - [ ] SubTask 5.4: 实现 POST /api/v1/decrypt 接口（支持指定算法和模式）
  - [ ] SubTask 5.5: 实现 POST /api/v1/keys/rotate 接口
  - [ ] SubTask 5.6: 实现 GET /api/v1/algorithms 接口（返回支持的算法和模式列表）

- [ ] Task 6: 部署架构支持（internal/config/ + cmd/）
  - [ ] SubTask 6.1: 实现配置管理（节点角色 standalone/key-server/worker、TPM 设备路径、监听端口、密钥服务地址列表）
  - [ ] SubTask 6.2: 实现单机模式（standalone）：密钥服务 + 加解密服务在同一进程
  - [ ] SubTask 6.3: 实现密钥服务模式（key-server）：仅提供密钥管理 API，连接 TPM
  - [ ] SubTask 6.4: 实现工作节点模式（worker）：通过远程密钥服务获取数据密钥，本地执行加解密
  - [ ] SubTask 6.5: 实现工作节点的密钥服务故障切换（多地址轮询）
  - [ ] SubTask 6.6: 实现 main.go 入口，根据配置加载对应模块

- [ ] Task 7: 集成测试与验证
  - [ ] SubTask 7.1: 编写根密钥创建/加载的单元测试
  - [ ] SubTask 7.2: 编写数据密钥生成/加密/解密的单元测试
  - [ ] SubTask 7.3: 编写 AES 各模式的加解密单元测试
  - [ ] SubTask 7.4: 编写 SM4 各模式的加解密单元测试
  - [ ] SubTask 7.5: 编写端到端加解密流程的集成测试（使用 swtpm 模拟器）
  - [ ] SubTask 7.6: 编写密钥轮换流程的集成测试
  - [ ] SubTask 7.7: 编写集群部署模式的集成测试（key-server + worker）

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 1]
- [Task 5] depends on [Task 3, Task 4]
- [Task 6] depends on [Task 5]
- [Task 7] depends on [Task 6]
