# Tasks

- [ ] Task 1: 项目基础结构搭建
  - [ ] SubTask 1.1: 创建项目目录结构（src/tpm、src/crypto、src/service、include/ 等）
  - [ ] SubTask 1.2: 创建 CMakeLists.txt 构建配置，引入 TPM2 TSS 库依赖
  - [ ] SubTask 1.3: 创建基础类型定义和错误码枚举

- [ ] Task 2: TPM 2.0 根密钥管理模块
  - [ ] SubTask 2.1: 实现 TPM 上下文初始化（连接 TPM 设备、创建 TCTI 和 ESAPI 上下文）
  - [ ] SubTask 2.2: 实现根密钥创建（创建 SRK，设置固定 hierarchy）
  - [ ] SubTask 2.3: 实现根密钥加载（从持久化存储加载已有 SRK）
  - [ ] SubTask 2.4: 实现 TPM 不可用时的错误处理

- [ ] Task 3: 数据密钥管理模块
  - [ ] SubTask 3.1: 实现数据密钥生成（生成 256 位随机密钥）
  - [ ] SubTask 3.2: 实现使用根密钥加密数据密钥（TPM 内部加密，返回密文）
  - [ ] SubTask 3.3: 实现使用根密钥解密数据密钥（TPM 内部解密，返回明文）
  - [ ] SubTask 3.4: 实现加密数据密钥的持久化存储接口

- [ ] Task 4: 数据加解密模块
  - [ ] SubTask 4.1: 实现 AES-256-GCM 加密函数
  - [ ] SubTask 4.2: 实现 AES-256-GCM 解密函数
  - [ ] SubTask 4.3: 实现密文格式封装（包含 IV、认证标签、密文）

- [ ] Task 5: 统一加解密服务接口
  - [ ] SubTask 5.1: 实现 EncryptionService 初始化（自动连接 TPM、加载根密钥）
  - [ ] SubTask 5.2: 实现 encrypt 接口（TPM 解密数据密钥 + AES 加密）
  - [ ] SubTask 5.3: 实现 decrypt 接口（TPM 解密数据密钥 + AES 解密）
  - [ ] SubTask 5.4: 实现数据密钥轮换接口

- [ ] Task 6: 集成测试与验证
  - [ ] SubTask 6.1: 编写根密钥创建/加载的单元测试
  - [ ] SubTask 6.2: 编写数据密钥生成/加密/解密的单元测试
  - [ ] SubTask 6.3: 编写端到端加解密流程的集成测试
  - [ ] SubTask 6.4: 编写密钥轮换流程的集成测试

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 1]
- [Task 5] depends on [Task 3, Task 4]
- [Task 6] depends on [Task 5]
