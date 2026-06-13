# Embedded crypto library Makefile
# Target: SOC2018 / Cortex-A9 (bare-metal)
# Supports: AES-128/192/256 (ECB, CBC, GCM), SM4 (ECB, CBC, GCM)

# -------- Toolchain selection --------
# make                 -> use host gcc for testing
# make CROSS=arm        -> use arm-none-eabi-gcc for target

CROSS   ?=
PREFIX  ?= arm-none-eabi-

ifeq ($(CROSS),arm)
CC       = $(PREFIX)gcc
AR       = $(PREFIX)ar
OBJCOPY  = $(PREFIX)objcopy
else
CC       = gcc
AR       = ar
OBJCOPY  = objcopy
endif

# -------- Source / Header layout --------
INCLUDE_DIR  = include
SRC_DIR      = src

COMMON_SRCS  = $(SRC_DIR)/common/gf128.c  \
               $(SRC_DIR)/common/ghash.c
AES_SRCS     = $(SRC_DIR)/aes/aes_core.c  \
               $(SRC_DIR)/aes/aes_gcm.c
SM4_SRCS     = $(SRC_DIR)/sm4/sm4_core.c  \
               $(SRC_DIR)/sm4/sm4_gcm.c

LIB_SRCS     = $(COMMON_SRCS) $(AES_SRCS) $(SM4_SRCS)
LIB_OBJS     = $(LIB_SRCS:.c=.o)

LIB_NAME     = libcrypto.a
TEST_SRC     = test/test_crypto.c
TEST_BIN     = test/test_crypto

# -------- Compiler flags --------
COMMON_CFLAGS = -Wall -I$(INCLUDE_DIR) -std=c99 -O2

# Cortex-A9 / SOC2018 specific flags
ifeq ($(CROSS),arm)
CPU_CFLAGS  = -mcpu=cortex-a9 -mfpu=neon-vfpv3 -mfloat-abi=hard \
              -marm -mno-unaligned-access
DEFS        = -D__ARMV7__ -D__SOC2018__ -DDEBUG -DOTHSEU -DTEST_ELF=2 -DUPLOAD
LDFLAGS     = -nostartfiles -nostdlib --specs=nosys.specs
else
CPU_CFLAGS  =
DEFS        = -DTEST_HOST -DDEBUG
LDFLAGS     =
endif

CFLAGS      = $(COMMON_CFLAGS) $(CPU_CFLAGS) $(DEFS)

# -------- Default target --------
all: $(LIB_NAME)

$(LIB_NAME): $(LIB_OBJS)
	$(AR) rcs $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

# -------- Host testing --------
test: $(LIB_NAME) $(TEST_BIN)
	@echo "----- Running host-side test -----"
	./$(TEST_BIN)

$(TEST_BIN): $(TEST_SRC) $(LIB_NAME)
	$(CC) $(COMMON_CFLAGS) $(TEST_SRC) $(LIB_NAME) -o $@

# -------- Cross-compile sanity build --------
cross:
	$(MAKE) clean
	$(MAKE) CROSS=arm $(LIB_NAME)

# -------- Cleanup --------
clean:
	rm -f $(LIB_OBJS) $(LIB_NAME) $(TEST_BIN)

.PHONY: all test cross clean
