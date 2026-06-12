/*
 *  Copyright 2014-2022 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 */

#include <stdint.h>
#include <stddef.h>
#include "gmssl_mem.h"

void memxor(void *r, const void *a, size_t len)
{
	uint8_t *pr = (uint8_t *)r;
	const uint8_t *pa = (const uint8_t *)a;
	size_t i;
	for (i = 0; i < len; i++) {
		pr[i] ^= pa[i];
	}
}

void gmssl_memxor(void *r, const void *a, const void *b, size_t len)
{
	uint8_t *pr = (uint8_t *)r;
	const uint8_t *pa = (const uint8_t *)a;
	const uint8_t *pb = (const uint8_t *)b;
	size_t i;
	for (i = 0; i < len; i++) {
		pr[i] = pa[i] ^ pb[i];
	}
}
