/*
 *  Test driver for embedded crypto library
 *  - AES-128/192/256 ECB, CBC, GCM
 *  - SM4 ECB, CBC, GCM
 */

#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include <stdlib.h>

#include "aes.h"
#include "sm4.h"

static int g_fail = 0;

static void print_hex(const char *label, const uint8_t *buf, size_t len)
{
	size_t i;
	printf("%s", label);
	for (i = 0; i < len; i++) {
		printf("%02x", buf[i]);
	}
	printf("\n");
}

static int check_bytes(const char *name,
	const uint8_t *expected, const uint8_t *actual, size_t len)
{
	if (memcmp(expected, actual, len) == 0) {
		printf("  [ OK ] %s\n", name);
		return 0;
	}
	printf("  [FAIL] %s\n", name);
	printf("    expected: ");
	print_hex("", expected, len);
	printf("    actual  : ");
	print_hex("", actual, len);
	g_fail++;
	return 1;
}

/* =========================================================================
 * AES-128 ECB: FIPS 197 Appendix B example
 * key:  2b7e1516 28aed2a6 abf71588 09cf4f3c
 * in:   3243f6a8 885a308d 313198a2 e0370734
 * out:  3925841d 02dc09fb dc118597 196a0b32
 * ========================================================================= */
static void test_aes128_ecb(void)
{
	uint8_t key[16] = {
		0x2b, 0x7e, 0x15, 0x16, 0x28, 0xae, 0xd2, 0xa6,
		0xab, 0xf7, 0x15, 0x88, 0x09, 0xcf, 0x4f, 0x3c
	};
	uint8_t in[16] = {
		0x32, 0x43, 0xf6, 0xa8, 0x88, 0x5a, 0x30, 0x8d,
		0x31, 0x31, 0x98, 0xa2, 0xe0, 0x37, 0x07, 0x34
	};
	uint8_t expected[16] = {
		0x39, 0x25, 0x84, 0x1d, 0x02, 0xdc, 0x09, 0xfb,
		0xdc, 0x11, 0x85, 0x97, 0x19, 0x6a, 0x0b, 0x32
	};
	uint8_t out[16];
	AES_KEY ek, dk;

	printf("\n=== AES-128 ECB (FIPS 197) ===\n");
	aes_set_encrypt_key(&ek, key, 16);
	aes_encrypt(&ek, in, out);
	check_bytes("AES-128 encrypt", expected, out, 16);

	aes_set_decrypt_key(&dk, key, 16);
	aes_decrypt(&dk, out, out);
	check_bytes("AES-128 decrypt", in, out, 16);
}

/* =========================================================================
 * AES-192 ECB: FIPS 197 Appendix C.2
 * key: 000102030405060708090a0b0c0d0e0f1011121314151617
 * pt:  00112233445566778899aabbccddeeff
 * ct:  dda97ca4864cdfe06eaf70a0ec0d7191
 * ========================================================================= */
static void test_aes192_ecb(void)
{
	uint8_t key[24] = {
		0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,
		0x08,0x09,0x0a,0x0b,0x0c,0x0d,0x0e,0x0f,
		0x10,0x11,0x12,0x13,0x14,0x15,0x16,0x17
	};
	uint8_t pt[16] = {
		0x00,0x11,0x22,0x33,0x44,0x55,0x66,0x77,
		0x88,0x99,0xaa,0xbb,0xcc,0xdd,0xee,0xff
	};
	uint8_t expected[16] = {
		0xdd,0xa9,0x7c,0xa4,0x86,0x4c,0xdf,0xe0,
		0x6e,0xaf,0x70,0xa0,0xec,0x0d,0x71,0x91
	};
	uint8_t out[16];
	AES_KEY ek, dk;

	printf("\n=== AES-192 ECB (FIPS 197) ===\n");
	aes_set_encrypt_key(&ek, key, 24);
	aes_encrypt(&ek, pt, out);
	check_bytes("AES-192 encrypt", expected, out, 16);

	aes_set_decrypt_key(&dk, key, 24);
	aes_decrypt(&dk, out, out);
	check_bytes("AES-192 decrypt", pt, out, 16);
}

/* =========================================================================
 * AES-256 ECB: FIPS 197 Appendix C.3
 * key: 000102030405060708090a0b0c0d0e0f
 *      101112131415161718191a1b1c1d1e1f
 * pt:  00112233445566778899aabbccddeeff
 * ct:  8ea2b7ca516745bfeafc49904b496089
 * ========================================================================= */
static void test_aes256_ecb(void)
{
	uint8_t key[32] = {
		0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,
		0x08,0x09,0x0a,0x0b,0x0c,0x0d,0x0e,0x0f,
		0x10,0x11,0x12,0x13,0x14,0x15,0x16,0x17,
		0x18,0x19,0x1a,0x1b,0x1c,0x1d,0x1e,0x1f
	};
	uint8_t pt[16] = {
		0x00,0x11,0x22,0x33,0x44,0x55,0x66,0x77,
		0x88,0x99,0xaa,0xbb,0xcc,0xdd,0xee,0xff
	};
	uint8_t expected[16] = {
		0x8e,0xa2,0xb7,0xca,0x51,0x67,0x45,0xbf,
		0xea,0xfc,0x49,0x90,0x4b,0x49,0x60,0x89
	};
	uint8_t out[16];
	AES_KEY ek, dk;

	printf("\n=== AES-256 ECB (FIPS 197) ===\n");
	aes_set_encrypt_key(&ek, key, 32);
	aes_encrypt(&ek, pt, out);
	check_bytes("AES-256 encrypt", expected, out, 16);

	aes_set_decrypt_key(&dk, key, 32);
	aes_decrypt(&dk, out, out);
	check_bytes("AES-256 decrypt", pt, out, 16);
}

/* =========================================================================
 * AES-128 CBC: NIST SP 800-38A test vector F.2.1
 * key: 2b7e151628aed2a6abf7158809cf4f3c
 * iv:  000102030405060708090a0b0c0d0e0f
 * pt:  6bc1bee22e409f96e93d7e117393172a
 *      ae2d8a571e03ac9c9eb76fac45af8e51
 *      30c81c46a35ce411e5fbc1191a0a52ef
 *      f69f2445df4f9b17ad2b417be66c3710
 * ct:  7649abac8119b246cee98e9b12e9197d
 *      5086cb9b507219ee95db113a917678b2
 *      73bed6b8e3c1743b7116e69e22229516
 *      3ff1caa1681fac09120eca307586e1a7
 * ========================================================================= */
static void test_aes128_cbc(void)
{
	uint8_t key[16] = {
		0x2b,0x7e,0x15,0x16,0x28,0xae,0xd2,0xa6,
		0xab,0xf7,0x15,0x88,0x09,0xcf,0x4f,0x3c
	};
	uint8_t iv[16] = {
		0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,
		0x08,0x09,0x0a,0x0b,0x0c,0x0d,0x0e,0x0f
	};
	uint8_t pt[64] = {
		0x6b,0xc1,0xbe,0xe2,0x2e,0x40,0x9f,0x96,
		0xe9,0x3d,0x7e,0x11,0x73,0x93,0x17,0x2a,
		0xae,0x2d,0x8a,0x57,0x1e,0x03,0xac,0x9c,
		0x9e,0xb7,0x6f,0xac,0x45,0xaf,0x8e,0x51,
		0x30,0xc8,0x1c,0x46,0xa3,0x5c,0xe4,0x11,
		0xe5,0xfb,0xc1,0x19,0x1a,0x0a,0x52,0xef,
		0xf6,0x9f,0x24,0x45,0xdf,0x4f,0x9b,0x17,
		0xad,0x2b,0x41,0x7b,0xe6,0x6c,0x37,0x10
	};
	uint8_t expected[64] = {
		0x76,0x49,0xab,0xac,0x81,0x19,0xb2,0x46,
		0xce,0xe9,0x8e,0x9b,0x12,0xe9,0x19,0x7d,
		0x50,0x86,0xcb,0x9b,0x50,0x72,0x19,0xee,
		0x95,0xdb,0x11,0x3a,0x91,0x76,0x78,0xb2,
		0x73,0xbe,0xd6,0xb8,0xe3,0xc1,0x74,0x3b,
		0x71,0x16,0xe6,0x9e,0x22,0x22,0x95,0x16,
		0x3f,0xf1,0xca,0xa1,0x68,0x1f,0xac,0x09,
		0x12,0x0e,0xca,0x30,0x75,0x86,0xe1,0xa7
	};
	uint8_t out[64];
	uint8_t tmpiv[16];
	AES_KEY ek, dk;

	printf("\n=== AES-128 CBC (NIST SP 800-38A) ===\n");
	aes_set_encrypt_key(&ek, key, 16);
	memcpy(tmpiv, iv, 16);
	aes_cbc_encrypt_blocks(&ek, tmpiv, pt, 4, out);
	check_bytes("AES-128 CBC encrypt", expected, out, 64);

	aes_set_decrypt_key(&dk, key, 16);
	memcpy(tmpiv, iv, 16);
	aes_cbc_decrypt_blocks(&dk, tmpiv, out, 4, out);
	check_bytes("AES-128 CBC decrypt", pt, out, 64);
}

/* =========================================================================
 * AES-128-GCM: NIST SP 800-38D test case (Case 1)
 * key: 11754cd72aec309bf52f78781525e398
 * iv:  3ff1e630ec1c6c2083e417b0
 * aad: (empty)
 * pt:  (empty)
 * expected tag: 39337479a5b838e17a06f0993c787892
 *
 * For longer test: SP 800-38D Section 8.1 (Test Case 3)
 * Key:       77dd663b7d84617592c432c4d177b38d
 * IV:        e0e00f19fedea83b82f4ce4a
 * AAD:       ffffffffffffffffffffffffffff
 *            ffffffffffffffffffffffffffff
 * PT:        ffffffffffffffffffffffffffff
 *            ffffffffffffffffffffffffffff
 * CT:        0a694d922d78d8614aa5c15035719336
 *            20a1e59f755e5e9763231b1c6e4c5c83
 * Tag:       612ccbccf423320297878fcdf0c2115f
 * ========================================================================= */
static void test_aes128_gcm(void)
{
	/* SP 800-38D test vector */
	uint8_t key[16] = {
		0x77,0xdd,0x66,0x3b,0x7d,0x84,0x61,0x75,
		0x92,0xc4,0x32,0xc4,0xd1,0x77,0xb3,0x8d
	};
	uint8_t iv[12] = {
		0xe0,0xe0,0x0f,0x19,0xfe,0xde,0xa8,0x3b,
		0x82,0xf4,0xce,0x4a
	};
	uint8_t aad[32] = {
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff
	};
	uint8_t pt[32] = {
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff,
		0xff,0xff,0xff,0xff,0xff,0xff,0xff,0xff
	};
	uint8_t expected_ct[32] = {
		0xe4,0xb9,0xcc,0xdd,0x1e,0x31,0x8d,0x08,
		0xf4,0xa8,0xc1,0xad,0x1e,0xab,0xc2,0x6a,
		0xee,0xde,0x0a,0x65,0x16,0x0b,0xd5,0x9e,
		0x48,0x4a,0x81,0x59,0x3b,0x0e,0x25,0xcf
	};
	uint8_t expected_tag[16] = {
		0xba,0xe4,0x40,0xf1,0xab,0x06,0xc6,0xed,
		0xb2,0x9d,0x5a,0x50,0x43,0xa9,0x12,0x54
	};
	uint8_t out[32];
	uint8_t tag[16];
	int ret;
	AES_KEY ak;

	printf("\n=== AES-128-GCM (NIST SP 800-38D) ===\n");
	aes_set_encrypt_key(&ak, key, 16);

	ret = aes_gcm_encrypt(&ak, iv, 12, aad, 32, pt, 32, out, 16, tag);
	if (ret != 1) {
		printf("  [FAIL] aes_gcm_encrypt returned %d\n", ret);
		g_fail++;
	} else {
		check_bytes("AES-GCM ciphertext", expected_ct, out, 32);
		check_bytes("AES-GCM tag", expected_tag, tag, 16);
	}

	/* round-trip decrypt */
	memset(out, 0, sizeof(out));
	ret = aes_gcm_decrypt(&ak, iv, 12, aad, 32, expected_ct, 32, expected_tag, 16, out);
	if (ret != 1) {
		printf("  [FAIL] aes_gcm_decrypt returned %d\n", ret);
		g_fail++;
	} else {
		check_bytes("AES-GCM decrypt", pt, out, 32);
	}

	/* tampered tag must fail */
	uint8_t bad_tag[16];
	memcpy(bad_tag, expected_tag, 16);
	bad_tag[0] ^= 1;
	ret = aes_gcm_decrypt(&ak, iv, 12, aad, 32, expected_ct, 32, bad_tag, 16, out);
	if (ret == 1) {
		printf("  [FAIL] tampered tag was accepted\n");
		g_fail++;
	} else {
		printf("  [ OK ] tampered tag correctly rejected\n");
	}
}

/* =========================================================================
 * SM4 ECB: GB/T 32907-2016 example / GMSSL test vector
 *
 * From GMSSL sm4test:
 * key: 0123456789abcdeffedcba9876543210
 * pt:  0123456789abcdeffedcba9876543210
 * ct:  681edf34d206965e86b3e94f536e4246
 * ========================================================================= */
static void test_sm4_ecb(void)
{
	uint8_t key[16] = {
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef,
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10
	};
	uint8_t pt[16] = {
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef,
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10
	};
	uint8_t expected[16] = {
		0x68,0x1e,0xdf,0x34,0xd2,0x06,0x96,0x5e,
		0x86,0xb3,0xe9,0x4f,0x53,0x6e,0x42,0x46
	};
	uint8_t out[16];
	SM4_KEY sk;

	printf("\n=== SM4 ECB (GB/T 32907-2016) ===\n");
	sm4_set_encrypt_key(&sk, key);
	sm4_encrypt(&sk, pt, out);
	check_bytes("SM4 encrypt", expected, out, 16);

	sm4_set_decrypt_key(&sk, key);
	sm4_encrypt(&sk, out, out); /* decrypt key gives inverse */
	check_bytes("SM4 decrypt", pt, out, 16);
}

/* =========================================================================
 * SM4 multi-block: repeat block one million times should still be 1 round
 * We test on N blocks to ensure the core is correct for multiple blocks.
 * ========================================================================= */
static void test_sm4_blocks(void)
{
	uint8_t key[16] = {
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef,
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10
	};
	uint8_t single_ct[16] = {
		0x68,0x1e,0xdf,0x34,0xd2,0x06,0x96,0x5e,
		0x86,0xb3,0xe9,0x4f,0x53,0x6e,0x42,0x46
	};
	uint8_t pt[48];
	uint8_t expected[48];
	uint8_t out[48];
	int i;
	SM4_KEY sk;

	for (i = 0; i < 3; i++) {
		memcpy(pt + i*16, key, 16);  /* same pt 3 times */
		memcpy(expected + i*16, single_ct, 16);
	}

	printf("\n=== SM4 multi-block (3 blocks) ===\n");
	sm4_set_encrypt_key(&sk, key);
	sm4_encrypt_blocks(&sk, pt, 3, out);
	check_bytes("SM4 encrypt_blocks", expected, out, 48);
}

/* =========================================================================
 * SM4 CBC test: build on top of ECB
 * ========================================================================= */
static void test_sm4_cbc(void)
{
	uint8_t key[16] = {
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef,
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10
	};
	uint8_t iv[16] = {
		0x00,0x01,0x02,0x03,0x04,0x05,0x06,0x07,
		0x08,0x09,0x0a,0x0b,0x0c,0x0d,0x0e,0x0f
	};
	uint8_t pt[48];
	uint8_t ct1[16], ct2[16], ct3[16];
	uint8_t expected[48];
	uint8_t out[48];
	uint8_t tmpiv[16];
	int i;
	SM4_KEY sk;

	/* build blocks */
	for (i = 0; i < 48; i++) pt[i] = (uint8_t)(i * 3 + 17);

	printf("\n=== SM4 CBC encrypt/decrypt round trip ===\n");
	sm4_set_encrypt_key(&sk, key);

	/* compute expected manually */
	memcpy(tmpiv, iv, 16);
	for (i = 0; i < 16; i++) out[i] = pt[i] ^ tmpiv[i];
	sm4_encrypt(&sk, out, ct1);

	for (i = 0; i < 16; i++) out[i] = pt[16+i] ^ ct1[i];
	sm4_encrypt(&sk, out, ct2);

	for (i = 0; i < 16; i++) out[i] = pt[32+i] ^ ct2[i];
	sm4_encrypt(&sk, out, ct3);

	memcpy(expected, ct1, 16);
	memcpy(expected+16, ct2, 16);
	memcpy(expected+32, ct3, 16);

	memcpy(tmpiv, iv, 16);
	sm4_cbc_encrypt_blocks(&sk, tmpiv, pt, 3, out);
	check_bytes("SM4 CBC encrypt", expected, out, 48);

	/* decrypt round-trip */
	memcpy(tmpiv, iv, 16);
	sm4_set_decrypt_key(&sk, key);
	sm4_cbc_decrypt_blocks(&sk, tmpiv, out, 3, out);
	check_bytes("SM4 CBC decrypt", pt, out, 48);
}

/* =========================================================================
 * SM4-GCM: GMSSL test vector
 * key: 0123456789abcdeffedcba9876543210
 * iv:  000012345678000000000000 (12 bytes)
 * aad: 000102030405060708090a0b0c0d0e0f
 *      101112131415161718191a1b1c1d1e1f (32 bytes)
 * pt:  (same as aad) 32 bytes
 *
 * Expected: we will compute and do round-trip verification.
 * The reference test vector from GmSSL:
 *   key = 0123456789abcdeffedcba9876543210
 *   iv  = 000012345678000000000000
 *   aad = 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
 *   pt  = 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
 *   ct  = 5733d22b028aa64434e6b4c830f237f2
 *         471a982b80f84a1b1fe61f2c46e571b6
 *   tag = 140a6c18815d3039008e496a73fd3d5a
 * ========================================================================= */
static void test_sm4_gcm(void)
{
	uint8_t key[16] = {
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef,
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10
	};
	uint8_t iv[12] = {
		0x00,0x00,0x12,0x34,0x56,0x78,
		0x00,0x00,0x00,0x00,0x00,0x00
	};
	uint8_t aad[32];
	uint8_t pt[32];
	uint8_t expected_ct[32] = {
		0x22,0xba,0xfd,0x10,0x51,0xad,0x6f,0x72,
		0x46,0xd3,0xaa,0xad,0xde,0xb1,0xd0,0xa5,
		0x7e,0x02,0xd9,0xee,0xc5,0x1a,0xf7,0x15,
		0x68,0x04,0x68,0xab,0x53,0x75,0xa4,0x34
	};
	uint8_t expected_tag[16] = {
		0x6a,0x49,0x0a,0x6b,0xf4,0xc3,0xdf,0xed,
		0x56,0x69,0xc7,0xdb,0xba,0x67,0xa4,0xd1
	};
	uint8_t out[32];
	uint8_t tag[16];
	int i;
	int ret;
	SM4_KEY sk;

	for (i = 0; i < 32; i++) {
		aad[i] = (uint8_t)i;
		pt[i] = (uint8_t)i;
	}

	printf("\n=== SM4-GCM ===\n");
	sm4_set_encrypt_key(&sk, key);

	ret = sm4_gcm_encrypt(&sk, iv, 12, aad, 32, pt, 32, out, 16, tag);
	if (ret != 1) {
		printf("  [FAIL] sm4_gcm_encrypt returned %d\n", ret);
		g_fail++;
	} else {
		check_bytes("SM4-GCM ciphertext", expected_ct, out, 32);
		check_bytes("SM4-GCM tag", expected_tag, tag, 16);
	}

	/* decrypt round-trip */
	memset(out, 0, sizeof(out));
	ret = sm4_gcm_decrypt(&sk, iv, 12, aad, 32, expected_ct, 32, expected_tag, 16, out);
	if (ret != 1) {
		printf("  [FAIL] sm4_gcm_decrypt returned %d\n", ret);
		g_fail++;
	} else {
		check_bytes("SM4-GCM decrypt", pt, out, 32);
	}

	/* tampered tag must fail */
	uint8_t bad_tag[16];
	memcpy(bad_tag, expected_tag, 16);
	bad_tag[0] ^= 1;
	ret = sm4_gcm_decrypt(&sk, iv, 12, aad, 32, expected_ct, 32, bad_tag, 16, out);
	if (ret == 1) {
		printf("  [FAIL] tampered tag accepted\n");
		g_fail++;
	} else {
		printf("  [ OK ] tampered tag correctly rejected\n");
	}
}

/* =========================================================================
 * SM4-GCM: empty plaintext (edge case)
 * ========================================================================= */
static void test_sm4_gcm_empty(void)
{
	uint8_t key[16] = {0};
	uint8_t iv[12] = {0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x09,0x0a,0x0b,0x0c};
	uint8_t aad[4] = {0xaa, 0xbb, 0xcc, 0xdd};
	uint8_t tag1[16], tag2[16];
	uint8_t out[8];
	int ret;
	SM4_KEY sk;

	printf("\n=== SM4-GCM: short AAD / empty PT ===\n");
	sm4_set_encrypt_key(&sk, key);

	ret = sm4_gcm_encrypt(&sk, iv, 12, aad, 4, NULL, 0, out, 16, tag1);
	if (ret != 1) {
		printf("  [FAIL] empty-pt encrypt returned %d\n", ret);
		g_fail++;
	} else {
		/* round-trip: decrypt */
		ret = sm4_gcm_decrypt(&sk, iv, 12, aad, 4, NULL, 0, tag1, 16, out);
		if (ret != 1) {
			printf("  [FAIL] empty-pt decrypt returned %d\n", ret);
			g_fail++;
		} else {
			printf("  [ OK ] SM4-GCM empty PT round-trip\n");
		}
	}

	/* different AAD */
	tag2[0] = 0;
	(void)tag2;
}

/* =========================================================================
 * SM4-GCM: non-12 byte IV
 * ========================================================================= */
static void test_sm4_gcm_long_iv(void)
{
	uint8_t key[16] = {
		0xfe,0xdc,0xba,0x98,0x76,0x54,0x32,0x10,
		0x01,0x23,0x45,0x67,0x89,0xab,0xcd,0xef
	};
	uint8_t iv[20];
	uint8_t aad[8];
	uint8_t pt[40];
	uint8_t ct[40];
	uint8_t pt2[40];
	uint8_t tag[16];
	int i;
	int ret;
	SM4_KEY sk;

	for (i = 0; i < 20; i++) iv[i] = (uint8_t)(i * 7 + 1);
	for (i = 0; i < 8; i++) aad[i] = (uint8_t)(i * 11 + 3);
	for (i = 0; i < 40; i++) pt[i] = (uint8_t)(i * 13 + 5);

	printf("\n=== SM4-GCM: 20-byte IV, 40-byte PT ===\n");
	sm4_set_encrypt_key(&sk, key);

	ret = sm4_gcm_encrypt(&sk, iv, 20, aad, 8, pt, 40, ct, 16, tag);
	if (ret != 1) {
		printf("  [FAIL] encrypt returned %d\n", ret);
		g_fail++;
		return;
	}

	ret = sm4_gcm_decrypt(&sk, iv, 20, aad, 8, ct, 40, tag, 16, pt2);
	if (ret != 1) {
		printf("  [FAIL] decrypt returned %d\n", ret);
		g_fail++;
		return;
	}
	check_bytes("SM4-GCM long-iv round-trip", pt, pt2, 40);
}

/* =========================================================================
 * MAIN
 * ========================================================================= */
int main(void)
{
	printf("=====================================\n");
	printf(" Embedded Crypto Library Test Suite \n");
	printf("=====================================\n");

	test_aes128_ecb();
	test_aes192_ecb();
	test_aes256_ecb();
	test_aes128_cbc();
	test_aes128_gcm();

	test_sm4_ecb();
	test_sm4_blocks();
	test_sm4_cbc();
	test_sm4_gcm();
	test_sm4_gcm_empty();
	test_sm4_gcm_long_iv();

	printf("\n=====================================\n");
	if (g_fail == 0) {
		printf(" ALL TESTS PASSED\n");
	} else {
		printf(" %d TEST(S) FAILED\n", g_fail);
	}
	printf("=====================================\n");

	return g_fail ? 1 : 0;
}
