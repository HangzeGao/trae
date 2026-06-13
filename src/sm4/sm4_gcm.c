/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *
 *  SM4-GCM: authenticated encryption with associated data
 */

#include <string.h>
#include "sm4.h"
#include "ghash.h"

/* memxor: out = a xor b (len bytes) */
static void memxor(uint8_t *out, const uint8_t *a, const uint8_t *b, size_t len)
{
	while (len--) {
		*out++ = *a++ ^ *b++;
	}
}

/* inc32() in nist-sp800-38d: increment only low 32 bits */
static void ctr32_incr(uint8_t a[16])
{
	int i;
	for (i = 15; i >= 12; i--) {
		a[i]++;
		if (a[i]) break;
	}
}

int sm4_gcm_encrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	uint8_t *out, size_t taglen, uint8_t *tag)
{
	uint8_t H[16] = {0};
	uint8_t Y[16];
	uint8_t T[16];

	if (ivlen < SM4_GCM_IV_MIN_SIZE || ivlen > SM4_GCM_IV_MAX_SIZE) {
		return -1;
	}
	if (taglen < SM4_GCM_MIN_TAG_SIZE || taglen > SM4_GCM_MAX_TAG_SIZE) {
		return -1;
	}

	sm4_encrypt(key, H, H);

	if (ivlen == 12) {
		memcpy(Y, iv, 12);
		Y[12] = Y[13] = Y[14] = 0;
		Y[15] = 1;
	} else {
		ghash(H, NULL, 0, iv, ivlen, Y);
	}

	sm4_encrypt(key, Y, T);

	ctr32_incr(Y);
	sm4_ctr32_encrypt(key, Y, in, inlen, out);

	ghash(H, aad, aadlen, out, inlen, H);
	memxor(tag, T, H, taglen);

	memset(H, 0, sizeof(H));
	memset(Y, 0, sizeof(Y));
	memset(T, 0, sizeof(T));
	return 1;
}

int sm4_gcm_decrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	const uint8_t *tag, size_t taglen, uint8_t *out)
{
	uint8_t H[16] = {0};
	uint8_t Y[16];
	uint8_t T[16];

	if (ivlen < SM4_GCM_IV_MIN_SIZE || ivlen > SM4_GCM_IV_MAX_SIZE) {
		return -1;
	}
	if (taglen < SM4_GCM_MIN_TAG_SIZE || taglen > SM4_GCM_MAX_TAG_SIZE) {
		return -1;
	}

	sm4_encrypt(key, H, H);

	if (ivlen == 12) {
		memcpy(Y, iv, 12);
		Y[12] = Y[13] = Y[14] = 0;
		Y[15] = 1;
	} else {
		ghash(H, NULL, 0, iv, ivlen, Y);
	}

	ghash(H, aad, aadlen, in, inlen, H);

	sm4_encrypt(key, Y, T);
	memxor(T, T, H, taglen);
	if (memcmp(T, tag, taglen) != 0) {
		memset(H, 0, sizeof(H));
		memset(Y, 0, sizeof(Y));
		memset(T, 0, sizeof(T));
		return -1;
	}

	ctr32_incr(Y);
	sm4_ctr32_encrypt(key, Y, in, inlen, out);

	memset(H, 0, sizeof(H));
	memset(Y, 0, sizeof(Y));
	memset(T, 0, sizeof(T));
	return 1;
}
