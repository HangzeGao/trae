/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 */

#include <string.h>
#include "endian.h"
#include "gf128.h"

void gf128_set_zero(gf128_t r)
{
	r[0] = 0;
	r[1] = 0;
}

void gf128_set_one(gf128_t r)
{
	r[0] = 1;
	r[1] = 0;
}

void gf128_add(gf128_t r, const gf128_t a, const gf128_t b)
{
	r[0] = a[0] ^ b[0];
	r[1] = a[1] ^ b[1];
}

static uint64_t reverse_bits(uint64_t a)
{
	uint64_t r = 0;
	int i;

	for (i = 0; i < 63; i++) {
		r |= a & 1;
		r <<= 1;
		a >>= 1;
	}
	r |= a & 1;
	return r;
}

void gf128_from_bytes(gf128_t r, const uint8_t p[16])
{
	r[0] = reverse_bits(GETU64(p));
	r[1] = reverse_bits(GETU64(p + 8));
}

void gf128_to_bytes(const gf128_t a, uint8_t p[16])
{
	PUTU64(p, reverse_bits(a[0]));
	PUTU64(p + 8, reverse_bits(a[1]));
}

/*
 * GF(2^128) multiplication
 * Operates on bit-reflected representation
 * Uses shift-and-xor method
 */
void gf128_mul(gf128_t r, const gf128_t a, const gf128_t b)
{
	const uint64_t mask = (uint64_t)1 << 63;
	uint64_t b0 = b[0];
	uint64_t b1 = b[1];
	uint64_t r0 = 0;
	uint64_t r1 = 0;
	int i;

	for (i = 0; i < 64; i++) {
		if (r1 & mask) {
			r1 = r1 << 1 | r0 >> 63;
			r0 = r0 << 1;
			r0 ^= 0x87;
		} else {
			r1 = r1 << 1 | r0 >> 63;
			r0 = r0 << 1;
		}

		if (b1 & mask) {
			r1 ^= a[1];
			r0 ^= a[0];
		}

		b1 <<= 1;
	}
	for (i = 0; i < 64; i++) {
		if (r1 & mask) {
			r1 = r1 << 1 | r0 >> 63;
			r0 = r0 << 1;
			r0 ^= 0x87;
		} else {
			r1 = r1 << 1 | r0 >> 63;
			r0 = r0 << 1;
		}

		if (b0 & mask) {
			r1 ^= a[1];
			r0 ^= a[0];
		}

		b0 <<= 1;
	}

	r[0] = r0;
	r[1] = r1;
}

void gf128_mul_by_2(gf128_t r, const gf128_t a)
{
	const uint64_t mask = (uint64_t)1 << 63;

	if (a[1] & mask) {
		r[1] = a[1] << 1 | a[0] >> 63;
		r[0] = a[0] << 1;
		r[0] ^= 0x87;
	} else {
		r[1] = a[1] << 1 | a[0] >> 63;
		r[0] = a[0] << 1;
	}
}
