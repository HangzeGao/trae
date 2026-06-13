/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *
 *  AES-GCM: authenticated encryption with associated data
 */

#include <string.h>
#include "aes.h"
#include "ghash.h"

/* memxor: out = a xor b (len bytes) */
static void memxor(uint8_t *out, const uint8_t *a, const uint8_t *b, size_t len)
{
	while (len--) {
		*out++ = *a++ ^ *b++;
	}
}

int aes_gcm_encrypt(const AES_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	uint8_t *out, size_t taglen, uint8_t *tag)
{
	uint8_t H[16] = {0};
	uint8_t Y[16];
	uint8_t T[16];

	if (ivlen < AES_GCM_IV_MIN_SIZE || ivlen > AES_GCM_IV_MAX_SIZE) {
		return -1;
	}
	if (taglen < AES_GCM_MIN_TAG_SIZE || taglen > AES_GCM_MAX_TAG_SIZE) {
		return -1;
	}

	aes_encrypt(key, H, H);

	if (ivlen == 12) {
		memcpy(Y, iv, 12);
		Y[12] = Y[13] = Y[14] = 0;
		Y[15] = 1;
	} else {
		ghash(H, NULL, 0, iv, ivlen, Y);
	}

	aes_encrypt(key, Y, T);

	/* CTR32 with Y+1 starts */
	{
		uint8_t ctr[16];
		memcpy(ctr, Y, 16);
		/* increment low 32 bits */
		int i;
		for (i = 15; i >= 12; i--) {
			ctr[i]++;
			if (ctr[i]) break;
		}
		aes_ctr32_encrypt(key, ctr, in, inlen, out);
		memset(ctr, 0, sizeof(ctr));
	}

	ghash(H, aad, aadlen, out, inlen, H);
	memxor(tag, T, H, taglen);

	memset(H, 0, sizeof(H));
	memset(Y, 0, sizeof(Y));
	memset(T, 0, sizeof(T));
	return 1;
}

int aes_gcm_decrypt(const AES_KEY *key, const uint8_t *iv, size_t ivlen,
	const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
	const uint8_t *tag, size_t taglen, uint8_t *out)
{
	uint8_t H[16] = {0};
	uint8_t Y[16];
	uint8_t T[16];

	if (ivlen < AES_GCM_IV_MIN_SIZE || ivlen > AES_GCM_IV_MAX_SIZE) {
		return -1;
	}
	if (taglen < AES_GCM_MIN_TAG_SIZE || taglen > AES_GCM_MAX_TAG_SIZE) {
		return -1;
	}

	aes_encrypt(key, H, H);

	if (ivlen == 12) {
		memcpy(Y, iv, 12);
		Y[12] = Y[13] = Y[14] = 0;
		Y[15] = 1;
	} else {
		ghash(H, NULL, 0, iv, ivlen, Y);
	}

	ghash(H, aad, aadlen, in, inlen, H);

	aes_encrypt(key, Y, T);
	memxor(T, T, H, taglen);
	if (memcmp(T, tag, taglen) != 0) {
		memset(H, 0, sizeof(H));
		memset(Y, 0, sizeof(Y));
		memset(T, 0, sizeof(T));
		return -1;
	}

	/* CTR32 decrypt with Y+1 starts */
	{
		uint8_t ctr[16];
		memcpy(ctr, Y, 16);
		int i;
		for (i = 15; i >= 12; i--) {
			ctr[i]++;
			if (ctr[i]) break;
		}
		aes_ctr32_encrypt(key, ctr, in, inlen, out);
		memset(ctr, 0, sizeof(ctr));
	}

	memset(H, 0, sizeof(H));
	memset(Y, 0, sizeof(Y));
	memset(T, 0, sizeof(T));
	return 1;
}
