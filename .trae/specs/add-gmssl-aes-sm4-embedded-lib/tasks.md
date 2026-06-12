# Tasks

- [ ] Task 1: 创建Git开发分支
  - [ ] SubTask 1.1: 从当前分支创建 feature/gmssl-aes-sm4-embedded 分支

- [ ] Task 2: 搭建项目目录结构和Makefile
  - [ ] SubTask 2.1: 创建目录结构（include/、src/aes/、src/sm4/、test/）
  - [ ] SubTask 2.2: 创建Makefile，配置交叉编译工具链和全部编译参数
    - 编译器前缀: arm-none-eabi-
    - CPU: -mcpu=cortex-a9
    - FPU: -mfpu=neon-vfpv3
    - 浮点ABI: -mfloat-abi=hard
    - 优化: -O2
    - 预定义宏: -D__ARMV7__ -D__SOC2018__ -DDEBUG -DOTHSEU -DTEST_ELF=2 -DUPLOAD
    - 共同参数: -Wall -fdata-sections -ffunction-sections -Wundef -Wshadow -Wconversion -Wredundant-decls -Wunknown-pragmas -mno-unaligned-access -marm
    - 链接参数: -Wl,--gc-sections

- [ ] Task 3: 适配AES算法实现
  - [ ] SubTask 3.1: 从GmSSL 3.0+提取AES核心算法代码（aes_core.c / aes_locl.h等）
  - [ ] SubTask 3.2: 去除OS依赖，适配裸机环境
  - [ ] SubTask 3.3: 实现AES-ECB和AES-CBC模式的加解密接口
  - [ ] SubTask 3.4: 创建AES公开头文件，定义AES API

- [ ] Task 4: 适配SM4算法实现
  - [ ] SubTask 4.1: 从GmSSL 3.0+提取SM4核心算法代码（sm4.c / sm4.h等）
  - [ ] SubTask 4.2: 去除OS依赖，适配裸机环境
  - [ ] SubTask 4.3: 实现SM4-ECB和SM4-CBC模式的加解密接口
  - [ ] SubTask 4.4: 创建SM4公开头文件，定义SM4 API

- [ ] Task 5: 创建统一对外头文件
  - [ ] SubTask 5.1: 创建 crypto_lib.h，统一包含AES和SM4接口

- [ ] Task 6: 编写测试程序
  - [ ] SubTask 6.1: 编写AES-ECB/CBC加解密测试用例（使用标准测试向量）
  - [ ] SubTask 6.2: 编写SM4-ECB/CBC加解密测试用例（使用国标测试向量）

- [ ] Task 7: 验证交叉编译
  - [ ] SubTask 7.1: 使用指定工具链完整编译库和测试程序，确认无编译错误和警告

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 2]
- [Task 4] depends on [Task 2]
- [Task 3] and [Task 4] 可并行执行
- [Task 5] depends on [Task 3, Task 4]
- [Task 6] depends on [Task 5]
- [Task 7] depends on [Task 6]
