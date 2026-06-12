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

#ifndef GMSSL_MEM_H
#define GMSSL_MEM_H

#include <stdint.h>
#include <stddef.h>

void memxor(void *r, const void *a, size_t len);
void gmssl_memxor(void *r, const void *a, const void *b, size_t len);

#endif
