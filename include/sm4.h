/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 */

#ifndef GMSSL_SM4_H
#define GMSSL_SM4_H

#include <stdint.h>
#include <stddef.h>
#include "ghash.h"

#ifdef __cplusplus
extern "C" {
#endif

#define SM4_KEY_SIZE		(16)
#define SM4_BLOCK_SIZE	(16)
#define SM4_NUM_ROUNDS	(32)

typedef struct {
	uint32_t rk[SM4_NUM_ROUNDS];
} SM4_KEY;

void sm4_set_encrypt_key(SM4_KEY *key, const uint8_t raw_key[SM4_KEY_SIZE]);
void sm4_set_decrypt_key(SM4_KEY *key, const uint8_t raw_key[SM4_KEY_SIZE]);
void sm4_encrypt(const SM4_KEY *key, const uint8_t in[SM4_BLOCK_SIZE], uint8_t out[SM4_BLOCK_SIZE]);
void sm4_encrypt_blocks(const SM4_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_cbc_encrypt_blocks(const SM4_KEY *key, uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_cbc_decrypt_blocks(const SM4_KEY *key, uint8_t iv[SM4_BLOCK_SIZE],
	const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_ctr32_encrypt_blocks(const SM4_KEY *key, uint8_t ctr[16], const uint8_t *in, size_t nblocks, uint8_t *out);
void sm4_ctr32_encrypt(const SM4_KEY *key, uint8_t ctr[16], const uint8_t *in, size_t inlen, uint8_t *out);

/* GCM mode */
#define SM4_GCM_IV_MIN_SIZE		1
#define SM4_GCM_IV_MAX_SIZE		64
#define SM4_GCM_IV_DEFAULT_SIZE	12
#define SM4_GCM_MIN_AAD_SIZE		0
#define SM4_GCM_MAX_AAD_SIZE		((1<<24))
#define SM4_GCM_MIN_PLAINTEXT_SIZE	0
#define SM4_GCM_MAX_TAG_SIZE		16
#define SM4_GCM_MIN_TAG_SIZE		12
#define SM4_GCM_DEFAULT_TAG_SIZE	16

int sm4_gcm_encrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	uint8_t *out, size_t taglen, uint8_t *tag);

int sm4_gcm_decrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	const uint8_t *tag, size_t taglen, uint8_t *out);

#ifdef __cplusplus
}
#endif
#endif
