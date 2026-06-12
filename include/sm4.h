/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *  - Removed string.h and ghash.h dependency
 *  - Only ECB and CBC modes
 *  - Using small footprint SM4 (no large T-tables)
 */

#ifndef GMSSL_SM4_H
#define GMSSL_SM4_H

#include <stdint.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif


#define SM4_KEY_SIZE		(16)
#define SM4_BLOCK_SIZE		(16)
#define SM4_NUM_ROUNDS		(32)


typedef struct {
	uint32_t rk[SM4_NUM_ROUNDS];
} SM4_KEY;

void sm4_set_encrypt_key(SM4_KEY *key, const uint8_t raw_key[SM4_KEY_SIZE]);
void sm4_set_decrypt_key(SM4_KEY *key, const uint8_t raw_key[SM4_KEY_SIZE]);
void sm4_encrypt(const SM4_KEY *key, const uint8_t in[SM4_BLOCK_SIZE], uint8_t out[SM4_BLOCK_SIZE]);

/* SM4-ECB mode: encrypt/decrypt nblocks of data */
void sm4_ecb_encrypt(const SM4_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_ecb_decrypt(const SM4_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out);

/* SM4-CBC mode */
void sm4_cbc_encrypt_blocks(const SM4_KEY *key, uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_cbc_decrypt_blocks(const SM4_KEY *key, uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t nblocks, uint8_t *out);
int  sm4_cbc_padding_encrypt(const SM4_KEY *key, const uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t inlen, uint8_t *out, size_t *outlen);
int  sm4_cbc_padding_decrypt(const SM4_KEY *key, const uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t inlen, uint8_t *out, size_t *outlen);


#ifdef __cplusplus
}
#endif
#endif
