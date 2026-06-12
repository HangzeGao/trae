# GmSSL AES/SM4 Embedded Crypto Library Makefile
# Target: SOC2018 + Cortex-A9
# Compiler: gcc-arm-none-eabi-9_2

# Toolchain
CROSS_COMPILE = arm-none-eabi-
CC = $(CROSS_COMPILE)gcc
AR = $(CROSS_COMPILE)ar
OBJCOPY = $(CROSS_COMPILE)objcopy
SIZE = $(CROSS_COMPILE)size

# Target CPU and FPU
CPU_FLAGS = -mcpu=cortex-a9 -marm -mfpu=neon-vfpv3 -mfloat-abi=hard -mno-unaligned-access

# Predefined macros
DEFS = -D__ARMV7__ -D__SOC2018__ -DDEBUG -DOTHSEU -DTEST_ELF=2 -DUPLOAD

# Common compile flags
CFLAGS_COMMON = -Wall -fdata-sections -ffunction-sections -Wundef -Wshadow \
                -Wconversion -Wredundant-decls -Wunknown-pragmas

# Optimization
OPT_LEVEL = -O2

# Include paths
INCLUDES = -Iinclude

# Combined CFLAGS
CFLAGS = $(CPU_FLAGS) $(OPT_LEVEL) $(CFLAGS_COMMON) $(DEFS) $(INCLUDES)

# Linker flags
LDFLAGS = -Wl,--gc-sections

# Library name
LIB_NAME = libcrypto_embed.a

# Source files
AES_SRCS = src/aes/aes_core.c src/aes/aes_modes.c
SM4_SRCS = src/sm4/sm4_core.c src/sm4/sm4_modes.c
COMMON_SRCS = src/common/memxor.c
SRCS = $(AES_SRCS) $(SM4_SRCS) $(COMMON_SRCS)

# Object files
OBJS = $(SRCS:.c=.o)

# Test
TEST_SRC = test/test_crypto.c
TEST_BIN = test_crypto.elf

.PHONY: all clean lib test

all: lib

lib: $(LIB_NAME)

$(LIB_NAME): $(OBJS)
	$(AR) rcs $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

test: $(TEST_BIN)

$(TEST_BIN): $(TEST_SRC) $(LIB_NAME)
	$(CC) $(CFLAGS) $(LDFLAGS) $< -L. -lcrypto_embed -o $@
	$(SIZE) $@

clean:
	rm -f $(OBJS) $(LIB_NAME) $(TEST_BIN)
