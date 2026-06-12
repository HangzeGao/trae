# 基于GmSSL3.0的AES/SM4嵌入式加解密库 Spec

## Why
需要在SOC2018 + Cortex-A9嵌入式平台上提供AES和SM4对称加解密能力，基于GmSSL 3.0+的实现进行裁剪适配，生成可独立编译的轻量级嵌入式加密库。

## What Changes
- 从GmSSL 3.0+源码中提取AES和SM4核心算法实现，去除对操作系统API（stdio、stdlib文件IO等）的依赖
- 适配嵌入式裸机环境，确保所有代码可在无OS环境下运行
- 创建适配SOC2018/Cortex-A9的交叉编译Makefile，包含指定的编译器、CPU类型、FPU、预定义宏和编译参数
- 提供统一的加解密API接口（AES-ECB/CBC、SM4-ECB/CBC）
- 新建Git分支进行开发

## Impact
- Affected code: 新增嵌入式加密库全部代码
- 依赖: GmSSL 3.0+ 源码（仅提取，不引入完整GmSSL构建系统）

## ADDED Requirements

### Requirement: AES对称加解密功能
系统 SHALL 提供AES对称加解密功能，支持128/192/256位密钥，支持ECB和CBC工作模式。

#### Scenario: AES-ECB加解密
- **WHEN** 用户调用AES-ECB加密接口，传入密钥和明文
- **THEN** 系统返回对应的密文；调用解密接口可还原明文

#### Scenario: AES-CBC加解密
- **WHEN** 用户调用AES-CBC加密接口，传入密钥、IV和明文
- **THEN** 系统返回对应的密文；调用解密接口可还原明文

### Requirement: SM4对称加解密功能
系统 SHALL 提供SM4对称加解密功能，支持128位密钥，支持ECB和CBC工作模式。

#### Scenario: SM4-ECB加解密
- **WHEN** 用户调用SM4-ECB加密接口，传入密钥和明文
- **THEN** 系统返回对应的密文；调用解密接口可还原明文

#### Scenario: SM4-CBC加解密
- **WHEN** 用户调用SM4-CBC加密接口，传入密钥、IV和明文
- **THEN** 系统返回对应的密文；调用解密接口可还原明文

### Requirement: 嵌入式平台适配
系统 SHALL 适配SOC2018 + Cortex-A9嵌入式平台，满足以下编译约束：

#### Scenario: 交叉编译
- **WHEN** 使用指定工具链编译库
- **THEN** 编译器为 gcc-arm-none-eabi-9_2，前缀 arm-none-eabi-
- **THEN** CPU类型为 -mcpu=cortex-a9
- **THEN** FPU类型为 -mfpu=neon-vfpv3
- **THEN** 浮点ABI为 -mfloat-abi=hard
- **THEN** 优化等级为 -O2
- **THEN** 预定义宏包含 __ARMV7__ __SOC2018__ DEBUG OTHSEU TEST_ELF=2 UPLOAD
- **THEN** 共同编译参数包含 -Wall -fdata-sections -ffunction-sections -Wundef -Wshadow -Wconversion -Wredundant-decls -Wunknown-pragmas -mno-unaligned-access -marm

### Requirement: 裸机环境兼容
系统 SHALL 在无操作系统环境下运行，不依赖POSIX API、标准文件IO、动态内存分配。

#### Scenario: 无OS依赖
- **WHEN** 在裸机环境下链接并调用加解密接口
- **THEN** 不产生对stdio/stdlib/POSIX的依赖，仅依赖基本C运行时

### Requirement: Git分支管理
系统 SHALL 在新建的Git分支上进行开发。

#### Scenario: 分支创建
- **WHEN** 开始开发
- **THEN** 从当前分支创建新分支 feature/gmssl-aes-sm4-embedded
