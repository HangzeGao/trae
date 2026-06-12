/*
 *  Copyright 2014-2026 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *  - Only ECB and CBC modes
 *  - Replaced gmssl headers with local headers
 *  - Using small footprint SM4 (no large T-tables)
 */

#include <string.h>
#include "sm4.h"
#include "gmssl_mem.h"
#include "gmssl_error.h"


/* SM4-ECB encrypt nblocks */
void sm4_ecb_encrypt(const SM4_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		sm4_encrypt(key, in, out);
		in += SM4_BLOCK_SIZE;
		out += SM4_BLOCK_SIZE;
	}
}

/* SM4-ECB decrypt nblocks */
void sm4_ecb_decrypt(const SM4_KEY *key, const uint8_t *in, size_t nblocks, uint8_t *out)
{
	while (nblocks--) {
		sm4_encrypt(key, in, out);
		in += SM4_BLOCK_SIZE;
		out += SM4_BLOCK_SIZE;
	}
}

void sm4_cbc_encrypt_blocks(const SM4_KEY *key, uint8_t iv[16],
	const uint8_t *in, size_t nblocks, uint8_t *out)
{
	const uint8_t *piv = iv;

	while (nblocks--) {
		size_t i;
		for (i = 0; i < 16; i++) {
			out[i] = in[i] ^ piv[i];
		}
		sm4_encrypt(key, out, out);
		piv = out;
		in += 16;
		out += 16;
	}

	memcpy(iv, piv, 16);
}

void sm4_cbc_decrypt_blocks(const SM4_KEY *key, uint8_t iv[16],
	const uint8_t *in, size_t nblocks, uint8_t *out)
{
	const uint8_t *piv = iv;

	while (nblocks--) {
		size_t i;
		sm4_encrypt(key, in, out);
		for (i = 0; i < 16; i++) {
			out[i] ^= piv[i];
		}
		piv = in;
		in += 16;
		out += 16;
	}

	memcpy(iv, piv, 16);
}

int sm4_cbc_padding_encrypt(const SM4_KEY *key, const uint8_t piv[16],
	const uint8_t *in, size_t inlen,
	uint8_t *out, size_t *outlen)
{
	uint8_t iv[16];
	uint8_t block[16];
	size_t rem = inlen % 16;
	int padding = 16 - (int)(inlen % 16);

	memcpy(iv, piv, 16);

	if (in) {
		memcpy(block, in + inlen - rem, rem);
	}
	memset(block + rem, padding, (size_t)padding);

	if (inlen/16) {
		sm4_cbc_encrypt_blocks(key, iv, in, inlen/16, out);
		out += inlen - rem;
	}
	sm4_cbc_encrypt_blocks(key, iv, block, 1, out);
	*outlen = inlen - rem + 16;
	return 1;
}

int sm4_cbc_padding_decrypt(const SM4_KEY *key, const uint8_t piv[16],
	const uint8_t *in, size_t inlen,
	uint8_t *out, size_t *outlen)
{
	uint8_t iv[16];
	uint8_t block[16];
	size_t len = sizeof(block);
	int padding;
	int i;

	memcpy(iv, piv, 16);

	if (inlen == 0) {
		error_puts("warning: input length = 0");
		return 0;
	}
	if (inlen%16 != 0 || inlen < 16) {
		error_puts("invalid cbc ciphertext length");
		return -1;
	}
	if (inlen > 16) {
		sm4_cbc_decrypt_blocks(key, iv, in, inlen/16 - 1, out);
	}

	sm4_cbc_decrypt_blocks(key, iv, in + inlen - 16, 1, block);

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
