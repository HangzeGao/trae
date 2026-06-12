# 基于 TPM 2.0 的软硬结合加解密系统 Spec

## Why
当前系统缺乏基于硬件安全模块的加解密能力，数据密钥以明文形式存储在内存或磁盘中，存在泄露风险。利用 TPM 2.0 芯片的硬件安全特性，可以实现根密钥永不离开芯片、数据密钥受根密钥保护的两层密钥体系，显著提升密钥安全性。

## What Changes
- 新增 TPM 2.0 根密钥（Root Key）管理模块：在 TPM 芯片内创建/加载根密钥，根密钥私钥部分永不离开 TPM
- 新增数据密钥（Data Key）管理模块：生成随机数据密钥，使用根密钥加密后持久化存储，使用时由 TPM 解密到内存
- 新增数据加解密模块：使用数据密钥对业务数据进行 AES-256-GCM 加解密
- 新增统一的加解密服务接口（EncryptionService），对外提供密钥管理和加解密能力
- **BREAKING** 无，此为全新功能

## Impact
- Affected specs: 无已有规格受影响
- Affected code: 全新模块，不影响已有代码

## ADDED Requirements

### Requirement: TPM 2.0 根密钥管理
系统 SHALL 在 TPM 2.0 芯片内创建和管理根密钥（Storage Root Key），根密钥的私钥部分永不离开 TPM 芯片。

#### Scenario: 首次初始化根密钥
- **WHEN** 系统首次启动且 TPM 中不存在根密钥
- **THEN** 系统在 TPM 中创建新的根密钥（SRK），并返回根密钥句柄

#### Scenario: 加载已有根密钥
- **WHEN** 系统启动且 TPM 中已存在根密钥
- **THEN** 系统加载已有根密钥并返回句柄，不重复创建

#### Scenario: TPM 不可用
- **WHEN** TPM 设备不可用或驱动异常
- **THEN** 系统返回明确的错误信息，拒绝继续操作

### Requirement: 数据密钥管理
系统 SHALL 支持数据密钥的生成、加密存储和解密加载，数据密钥由根密钥加密保护。

#### Scenario: 生成数据密钥
- **WHEN** 用户请求生成新的数据密钥
- **THEN** 系统生成 256 位随机数据密钥，使用根密钥在 TPM 内加密数据密钥，返回加密后的数据密钥密文和明文数据密钥（仅本次返回）

#### Scenario: 解密数据密钥
- **WHEN** 用户需要使用已加密的数据密钥
- **THEN** 系统将加密的数据密钥密文送入 TPM，由根密钥解密，返回明文数据密钥供加解密使用

#### Scenario: 持久化数据密钥
- **WHEN** 用户需要保存数据密钥
- **THEN** 系统仅存储加密后的数据密钥密文，不存储明文数据密钥

### Requirement: 数据加解密
系统 SHALL 使用数据密钥对业务数据进行 AES-256-GCM 加解密操作。

#### Scenario: 加密数据
- **WHEN** 用户传入明文数据和加密后的数据密钥密文
- **THEN** 系统先通过 TPM 解密数据密钥，再使用数据密钥以 AES-256-GCM 算法加密数据，返回密文和认证标签

#### Scenario: 解密数据
- **WHEN** 用户传入密文、认证标签和加密后的数据密钥密文
- **THEN** 系统先通过 TPM 解密数据密钥，再使用数据密钥以 AES-256-GCM 算法解密数据，返回明文

#### Scenario: 数据密钥解密失败
- **WHEN** 数据密钥密文被篡改或与根密钥不匹配
- **THEN** TPM 解密失败，系统返回错误信息，拒绝加解密操作

### Requirement: 统一加解密服务接口
系统 SHALL 提供 EncryptionService 统一接口，封装密钥管理和加解密操作。

#### Scenario: 初始化服务
- **WHEN** 应用启动并初始化 EncryptionService
- **THEN** 服务自动连接 TPM、加载/创建根密钥，进入就绪状态

#### Scenario: 一站式加密
- **WHEN** 调用 EncryptionService.encrypt(data, encryptedDataKey)
- **THEN** 系统完成 TPM 解密数据密钥 + AES-256-GCM 加密，返回密文

#### Scenario: 一站式解密
- **WHEN** 调用 EncryptionService.decrypt(ciphertext, encryptedDataKey)
- **THEN** 系统完成 TPM 解密数据密钥 + AES-256-GCM 解密，返回明文

### Requirement: 密钥生命周期管理
系统 SHALL 支持数据密钥的轮换和废弃。

#### Scenario: 轮换数据密钥
- **WHEN** 用户请求轮换数据密钥
- **THEN** 系统生成新数据密钥，使用根密钥加密后返回，旧数据密钥标记为待废弃

#### Scenario: 使用新密钥重新加密数据
- **WHEN** 数据密钥轮换后需要重新加密已有数据
- **THEN** 系统使用旧数据密钥解密数据，再使用新数据密钥重新加密
