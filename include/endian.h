/*
 *  Copyright 2014-2024 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 */

#ifndef GMSSL_ENDIAN_H
#define GMSSL_ENDIAN_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define ROL32(A, n) (((A) << (n)) | (((A) >> (32 - (n))) & (((uint32_t)1 << (n)) - 1)))

static inline uint32_t getu32(const uint8_t *p)
{
	return ((uint32_t)p[0] << 24) | ((uint32_t)p[1] << 16) | ((uint32_t)p[2] << 8) | (uint32_t)p[3];
}

static inline void putu32(uint8_t *p, uint32_t v)
{
	p[0] = (uint8_t)(v >> 24);
	p[1] = (uint8_t)(v >> 16);
	p[2] = (uint8_t)(v >> 8);
	p[3] = (uint8_t)v;
}

static inline uint64_t getu64(const uint8_t *p)
{
	return ((uint64_t)getu32(p) << 32) | (uint64_t)getu32(p + 4);
}

static inline void putu64(uint8_t *p, uint64_t v)
{
	putu32(p, (uint32_t)(v >> 32));
	putu32(p + 4, (uint32_t)v);
}

#ifndef GETU32
#define GETU32(p) getu32(p)
#endif
#ifndef PUTU32
#define PUTU32(p, v) putu32(p, v)
#endif
#ifndef GETU64
#define GETU64(p) getu64(p)
#endif
#ifndef PUTU64
#define PUTU64(p, v) putu64(p, v)
#endif

#ifdef __cplusplus
}
#endif
#endif
