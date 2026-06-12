/*
 *  Copyright 2014-2025 The GmSSL Project. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the License); you may
 *  not use this file except in compliance with the License.
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Adapted for embedded bare-metal environment (SOC2018/Cortex-A9)
 *  - Removed stdio.h dependency (no fprintf)
 *  - error_print() becomes a no-op in bare-metal
 */

#ifndef GMSSL_ERROR_H
#define GMSSL_ERROR_H

/* Bare-metal compatible error macros - no-op */
#define error_print()       ((void)0)
#define error_puts(str)     ((void)0)
#define error_print_msg(fmt, ...) ((void)0)
#define warning_print()     ((void)0)

#endif
