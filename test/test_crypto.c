/*
 *  Test program for AES/SM4 embedded crypto library
 *  Uses standard test vectors for verification
 *
 *  Copyright 2014-2026 The GmSSL Project. All Rights Reserved.
 *  Licensed under the Apache License, Version 2.0
 */

#include <string.h>
#include <stdio.h>
#include <sys/stat.h>
#include "crypto_lib.h"

/* Bare-metal system call stubs for newlib */
int _write(int file, char *ptr, int len) { (void)file; (void)ptr; return len; }
int _read(int file, char *ptr, int len) { (void)file; (void)ptr; return len; }
int _close(int file) { (void)file; return -1; }
int _fstat(int file, struct stat *st) { (void)file; st->st_mode = 0; return 0; }
int _isatty(int file) { (void)file; return 1; }
int _lseek(int file, int ptr, int dir) { (void)file; (void)ptr; (void)dir; return 0; }
void *_sbrk(int incr) { (void)incr; return (void *)0; }
int _kill(int pid, int sig) { (void)pid; (void)sig; return -1; }
int _getpid(void) { return 1; }
void _exit(int status) { (void)status; while(1) {} }

/* Test framework */
static int g_test_pass = 0;
static int g_test_fail = 0;

static void check(const char *name, const uint8_t *actual, const uint8_t *expected, size_t len)
{
	size_t i;
	int ok = 1;
	for (i = 0; i < len; i++) {
		if (actual[i] != expected[i]) {
			ok = 0;
			break;
		}
	}
	if (ok) {
		g_test_pass++;
		printf("PASS: %s\n", name);
	} else {
		g_test_fail++;
		printf("FAIL: %s\n  actual:  ", name);
		for (i = 0; i < len; i++) printf("%02x", actual[i]);
		printf("\n  expect:  ");
		for (i = 0; i < len; i++) printf("%02x", expected[i]);
		printf("\n");
	}
}

/* AES-128 ECB test vector from FIPS 197 Appendix B */
static void test_aes128_ecb(void)
{
	AES_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x2b, 0x7e, 0x15, 0x16, 0x28, 0xae, 0xd2, 0xa6,
		0xab, 0xf7, 0x15, 0x88, 0x09, 0xcf, 0x4f, 0x3c
	};
	const uint8_t plaintext[16] = {
		0x32, 0x43, 0xf6, 0xa8, 0x88, 0x5a, 0x30, 0x8d,
		0x31, 0x31, 0x98, 0xa2, 0xe0, 0x37, 0x07, 0x34
	};
	const uint8_t expected_ct[16] = {
		0x39, 0x25, 0x84, 0x1d, 0x02, 0xdc, 0x09, 0xfb,
		0xdc, 0x11, 0x85, 0x97, 0x19, 0x6a, 0x0b, 0x32
	};
	uint8_t ciphertext[16];
	uint8_t decrypted[16];

	aes_set_encrypt_key(&enc_key, key, 16);
	aes_encrypt(&enc_key, plaintext, ciphertext);
	check("AES-128-ECB encrypt", ciphertext, expected_ct, 16);

	aes_set_decrypt_key(&dec_key, key, 16);
	aes_decrypt(&dec_key, ciphertext, decrypted);
	check("AES-128-ECB decrypt", decrypted, plaintext, 16);
}

/* AES-256 ECB test vector from NIST SP 800-38A */
static void test_aes256_ecb(void)
{
	AES_KEY enc_key, dec_key;
	const uint8_t key[32] = {
		0x60, 0x3d, 0xeb, 0x10, 0x15, 0xca, 0x71, 0xbe,
		0x2b, 0x73, 0xae, 0xf0, 0x85, 0x7d, 0x77, 0x81,
		0x1f, 0x35, 0x2c, 0x07, 0x3b, 0x61, 0x08, 0xd7,
		0x2d, 0x98, 0x10, 0xa3, 0x09, 0x14, 0xdf, 0xf4
	};
	const uint8_t plaintext[16] = {
		0x6b, 0xc1, 0xbe, 0xe2, 0x2e, 0x40, 0x9f, 0x96,
		0xe9, 0x3d, 0x7e, 0x11, 0x73, 0x93, 0x17, 0x2a
	};
	const uint8_t expected_ct[16] = {
		0xf3, 0xee, 0xd1, 0xbd, 0xb5, 0xd2, 0xa0, 0x3c,
		0x06, 0x4b, 0x5a, 0x7e, 0x3d, 0xb1, 0x81, 0xf8
	};
	uint8_t ciphertext[16];
	uint8_t decrypted[16];

	aes_set_encrypt_key(&enc_key, key, 32);
	aes_encrypt(&enc_key, plaintext, ciphertext);
	check("AES-256-ECB encrypt", ciphertext, expected_ct, 16);

	aes_set_decrypt_key(&dec_key, key, 32);
	aes_decrypt(&dec_key, ciphertext, decrypted);
	check("AES-256-ECB decrypt", decrypted, plaintext, 16);
}

/* AES-128 CBC test vector from NIST SP 800-38A F.2.1 */
static void test_aes128_cbc(void)
{
	AES_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x2b, 0x7e, 0x15, 0x16, 0x28, 0xae, 0xd2, 0xa6,
		0xab, 0xf7, 0x15, 0x88, 0x09, 0xcf, 0x4f, 0x3c
	};
	const uint8_t iv[16] = {
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f
	};
	const uint8_t plaintext[32] = {
		0x6b, 0xc1, 0xbe, 0xe2, 0x2e, 0x40, 0x9f, 0x96,
		0xe9, 0x3d, 0x7e, 0x11, 0x73, 0x93, 0x17, 0x2a,
		0xae, 0x2d, 0x8a, 0x57, 0x1e, 0x03, 0xac, 0x9c,
		0x9e, 0xb7, 0x6f, 0xac, 0x45, 0xaf, 0x8e, 0x51
	};
	const uint8_t expected_ct[32] = {
		0x76, 0x49, 0xab, 0xac, 0x81, 0x19, 0xb2, 0x46,
		0xce, 0xe9, 0x8e, 0x9b, 0x12, 0xe9, 0x19, 0x7d,
		0x50, 0x86, 0xcb, 0x9b, 0x50, 0x72, 0x19, 0xee,
		0x95, 0xdb, 0x11, 0x3a, 0x91, 0x76, 0x78, 0xb2
	};
	uint8_t ciphertext[32];
	uint8_t decrypted[32];

	aes_set_encrypt_key(&enc_key, key, 16);
	aes_cbc_encrypt(&enc_key, iv, plaintext, 2, ciphertext);
	check("AES-128-CBC encrypt", ciphertext, expected_ct, 32);

	aes_set_decrypt_key(&dec_key, key, 16);
	aes_cbc_decrypt(&dec_key, iv, ciphertext, 2, decrypted);
	check("AES-128-CBC decrypt", decrypted, plaintext, 32);
}

/* SM4 ECB test vector from GB/T 32907-2016 */
static void test_sm4_ecb(void)
{
	SM4_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	const uint8_t plaintext[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	const uint8_t expected_ct[16] = {
		0x68, 0x1e, 0xdf, 0x34, 0xd2, 0x06, 0x96, 0x5e,
		0x86, 0xb3, 0xe9, 0x4f, 0x53, 0x6e, 0x42, 0x46
	};
	uint8_t ciphertext[16];
	uint8_t decrypted[16];

	sm4_set_encrypt_key(&enc_key, key);
	sm4_encrypt(&enc_key, plaintext, ciphertext);
	check("SM4-ECB encrypt", ciphertext, expected_ct, 16);

	sm4_set_decrypt_key(&dec_key, key);
	sm4_encrypt(&dec_key, ciphertext, decrypted);
	check("SM4-ECB decrypt", decrypted, plaintext, 16);
}

/* SM4 repeated encryption test (1,000,000 times) from GB/T 32907-2016 */
static void test_sm4_repeat(void)
{
	SM4_KEY key;
	const uint8_t raw_key[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	const uint8_t expected[16] = {
		0x59, 0x52, 0x98, 0xc7, 0xc6, 0xfd, 0x27, 0x1f,
		0x04, 0x02, 0xf8, 0x04, 0xc3, 0x3d, 0x3f, 0x66
	};
	uint8_t block[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	int i;

	sm4_set_encrypt_key(&key, raw_key);
	for (i = 0; i < 1000000; i++) {
		sm4_encrypt(&key, block, block);
	}
	check("SM4 repeat 1M", block, expected, 16);
}

/* SM4 CBC test vector */
static void test_sm4_cbc(void)
{
	SM4_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	const uint8_t iv[16] = {
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f
	};
	const uint8_t plaintext[32] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	uint8_t ciphertext[32];
	uint8_t decrypted[32];
	uint8_t iv_enc[16], iv_dec[16];

	memcpy(iv_enc, iv, 16);
	memcpy(iv_dec, iv, 16);

	sm4_set_encrypt_key(&enc_key, key);
	sm4_cbc_encrypt_blocks(&enc_key, iv_enc, plaintext, 2, ciphertext);

	sm4_set_decrypt_key(&dec_key, key);
	sm4_cbc_decrypt_blocks(&dec_key, iv_dec, ciphertext, 2, decrypted);
	check("SM4-CBC decrypt", decrypted, plaintext, 32);
}

/* AES-128 CBC padding test */
static void test_aes128_cbc_padding(void)
{
	AES_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x2b, 0x7e, 0x15, 0x16, 0x28, 0xae, 0xd2, 0xa6,
		0xab, 0xf7, 0x15, 0x88, 0x09, 0xcf, 0x4f, 0x3c
	};
	const uint8_t iv[16] = {
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f
	};
	const uint8_t plaintext[20] = {
		0x6b, 0xc1, 0xbe, 0xe2, 0x2e, 0x40, 0x9f, 0x96,
		0xe9, 0x3d, 0x7e, 0x11, 0x73, 0x93, 0x17, 0x2a,
		0xae, 0x2d, 0x8a, 0x57
	};
	uint8_t ciphertext[32];
	uint8_t decrypted[32];
	size_t ct_len, pt_len;

	aes_set_encrypt_key(&enc_key, key, 16);
	aes_cbc_padding_encrypt(&enc_key, iv, plaintext, 20, ciphertext, &ct_len);

	aes_set_decrypt_key(&dec_key, key, 16);
	aes_cbc_padding_decrypt(&dec_key, iv, ciphertext, ct_len, decrypted, &pt_len);

	if (pt_len == 20) {
		check("AES-128-CBC padding", decrypted, plaintext, 20);
	} else {
		g_test_fail++;
		printf("FAIL: AES-128-CBC padding (pt_len=%zu, expected 20)\n", pt_len);
	}
}

/* SM4 CBC padding test */
static void test_sm4_cbc_padding(void)
{
	SM4_KEY enc_key, dec_key;
	const uint8_t key[16] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10
	};
	const uint8_t iv[16] = {
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f
	};
	const uint8_t plaintext[20] = {
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
		0xaa, 0xbb, 0xcc, 0xdd
	};
	uint8_t ciphertext[32];
	uint8_t decrypted[32];
	size_t ct_len, pt_len;

	sm4_set_encrypt_key(&enc_key, key);
	sm4_cbc_padding_encrypt(&enc_key, iv, plaintext, 20, ciphertext, &ct_len);

	sm4_set_decrypt_key(&dec_key, key);
	sm4_cbc_padding_decrypt(&dec_key, iv, ciphertext, ct_len, decrypted, &pt_len);

	if (pt_len == 20) {
		check("SM4-CBC padding", decrypted, plaintext, 20);
	} else {
		g_test_fail++;
		printf("FAIL: SM4-CBC padding (pt_len=%zu, expected 20)\n", pt_len);
	}
}

int main(void)
{
	printf("Starting tests...\n");
	test_aes128_ecb();
	test_aes256_ecb();
	test_aes128_cbc();
	test_aes128_cbc_padding();
	test_sm4_ecb();
	test_sm4_repeat();
	test_sm4_cbc();
	test_sm4_cbc_padding();

	printf("\nResults: %d passed, %d failed\n", g_test_pass, g_test_fail);

	return (g_test_fail > 0) ? 1 : 0;
}
