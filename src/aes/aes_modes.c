/*
 *  Copyright 2014-2026 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *  - Removed stdio.h and stdlib.h dependencies
 *  - Only ECB and CBC modes (removed CTR, GCM)
 *  - Replaced gmssl headers with local headers
 */

#include <string.h>
#include "aes.h"
#include "gmssl_mem.h"
#include "gmssl_error.h"


/* AES-ECB encrypt nblocks */
void aes_ecb_encrypt(const AES_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		aes_encrypt(key, in, out);
		in += AES_BLOCK_SIZE;
		out += AES_BLOCK_SIZE;
	}
}

/* AES-ECB decrypt nblocks */
void aes_ecb_decrypt(const AES_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		aes_decrypt(key, in, out);
		in += AES_BLOCK_SIZE;
		out += AES_BLOCK_SIZE;
	}
}

void aes_cbc_encrypt(const AES_KEY *key, const uint8_t iv[16],
	const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		gmssl_memxor(out, in, iv, 16);
		aes_encrypt(key, out, out);
		iv = out;
		in += 16;
		out += 16;
	}
}

void aes_cbc_decrypt(const AES_KEY *key, const uint8_t iv[16],
	const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		aes_decrypt(key, in, out);
		memxor(out, iv, 16);
		iv = in;
		in += 16;
		out += 16;
	}
}

int aes_cbc_padding_encrypt(const AES_KEY *key, const uint8_t iv[16],
	const uint8_t *in, size_t inlen,
	uint8_t *out, size_t *outlen)
{
	uint8_t block[16];
	size_t rem = inlen % 16;
	int padding = 16 - (int)(inlen % 16);

	if (in) {
		memcpy(block, in + inlen - rem, rem);
	}
	memset(block + rem, padding, (size_t)padding);
	if (inlen/16) {
		aes_cbc_encrypt(key, iv, in, inlen/16, out);
		out += inlen - rem;
		iv = out - 16;
	}
	aes_cbc_encrypt(key, iv, block, 1, out);
	*outlen = inlen - rem + 16;
	return 1;
}

int aes_cbc_padding_decrypt(const AES_KEY *key, const uint8_t iv[16],
	const uint8_t *in, size_t inlen,
	uint8_t *out, size_t *outlen)
{
	uint8_t block[16];
	size_t len = sizeof(block);
	int padding;
	int i;

	if (inlen == 0) {
		error_print();
		return 0;
	}
	if (inlen%16 != 0 || inlen < 16) {
		error_print();
		return -1;
	}
	if (inlen > 16) {
		aes_cbc_decrypt(key, iv, in, inlen/16 - 1, out);
		iv = in + inlen - 32;
	}
	aes_cbc_decrypt(key, iv, in + inlen - 16, 1, block);
	padding = block[15];
	if (padding < 1 || padding > 16) {
		error_print();
		return -1;
	}
	for (i = 16 - padding; i < 16; i++) {
		if (block[i] != (uint8_t)padding) {
			error_print();
			return -1;
		}
	}

	len -= (size_t)padding;
	memcpy(out + inlen - 16, block, len);
	*outlen = inlen - (size_t)padding;
	return 1;
}
