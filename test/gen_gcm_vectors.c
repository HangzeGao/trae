/* Build against GmSSL reference to get authoritative test vectors.
   Compile: gcc -I/workspace/gmssl_ref/include -std=c99 -O2 gen_gcm_vectors.c \
              /workspace/gmssl_ref/src/sm4.c /workspace/gmssl_ref/src/gf128.c \
              /workspace/gmssl_ref/src/ghash.c /workspace/gmssl_ref/src/sm4_gcm.c \
              /workspace/gmssl_ref/src/aes.c /workspace/gmssl_ref/src/aes_modes.c \
              /workspace/gmssl_ref/src/endian.h /workspace/gmssl_ref/src/mem.h -o gen
   But easier: use our own compiled lib */
#include <stdio.h>
#include <string.h>
#include <stdint.h>

/* Declare APIs directly */
typedef struct { uint32_t rk[60]; size_t rounds; } AES_KEY;
typedef struct { uint32_t rk[32]; } SM4_KEY;

int aes_set_encrypt_key(AES_KEY *key, const uint8_t *raw, size_t len);
int aes_set_decrypt_key(AES_KEY *key, const uint8_t *raw, size_t len);
void aes_encrypt(const AES_KEY *key, const uint8_t in[16], uint8_t out[16]);
void aes_decrypt(const AES_KEY *key, const uint8_t in[16], uint8_t out[16]);

int aes_gcm_encrypt(const AES_KEY *key, const uint8_t *iv, size_t ivlen,
                    const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
                    uint8_t *out, size_t taglen, uint8_t *tag);
int aes_gcm_decrypt(const AES_KEY *key, const uint8_t *iv, size_t ivlen,
                    const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
                    const uint8_t *tag, size_t taglen, uint8_t *out);

void sm4_set_encrypt_key(SM4_KEY *key, const uint8_t raw[16]);
void sm4_set_decrypt_key(SM4_KEY *key, const uint8_t raw[16]);

int sm4_gcm_encrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
                    const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
                    uint8_t *out, size_t taglen, uint8_t *tag);
int sm4_gcm_decrypt(const SM4_KEY *key, const uint8_t *iv, size_t ivlen,
                    const uint8_t *aad, size_t aadlen, const uint8_t *in, size_t inlen,
                    const uint8_t *tag, size_t taglen, uint8_t *out);

static void hexdump(const char *label, const uint8_t *buf, size_t len) {
    printf("%s", label);
    for (size_t i = 0; i < len; i++) printf("%02x", buf[i]);
    printf("\n");
}

int main(void) {
    /* ============== AES-128-GCM vector (NIST SP 800-38D) ==============
       Key: 77dd663b 7d846175 92c432c4 d177b38d
       IV:  e0e00f19 fedea83b 82f4ce4a
       AAD: ffffffffffffffffffffffffffffffff
            ffffffffffffffffffffffffffffffff
       PT:  ffffffffffffffffffffffffffffffff
            ffffffffffffffffffffffffffffffff
    */
    {
        uint8_t key[16] = {
            0x77,0xdd,0x66,0x3b, 0x7d,0x84,0x61,0x75,
            0x92,0xc4,0x32,0xc4, 0xd1,0x77,0xb3,0x8d
        };
        uint8_t iv[12] = {
            0xe0,0xe0,0x0f,0x19, 0xfe,0xde,0xa8,0x3b,
            0x82,0xf4,0xce,0x4a
        };
        uint8_t aad[32]; memset(aad, 0xff, 32);
        uint8_t pt[32];  memset(pt,  0xff, 32);
        uint8_t ct[32];
        uint8_t tag[16];

        AES_KEY ak;
        aes_set_encrypt_key(&ak, key, 16);
        int ret = aes_gcm_encrypt(&ak, iv, 12, aad, 32, pt, 32, ct, 16, tag);
        printf("AES-128-GCM ret=%d\n", ret);
        hexdump("  CT  = ", ct, 32);
        hexdump("  TAG = ", tag, 16);

        /* Round-trip decrypt */
        uint8_t out[32]; memset(out, 0, 32);
        ret = aes_gcm_decrypt(&ak, iv, 12, aad, 32, ct, 32, tag, 16, out);
        printf("  decrypt ret=%d, match=%d\n", ret, memcmp(out, pt, 32) == 0);
    }

    /* ============== SM4-GCM test vector ============== */
    {
        uint8_t key[16] = {
            0x01,0x23,0x45,0x67, 0x89,0xab,0xcd,0xef,
            0xfe,0xdc,0xba,0x98, 0x76,0x54,0x32,0x10
        };
        uint8_t iv[12] = {
            0x00,0x00,0x12,0x34, 0x56,0x78,0x00,0x00,
            0x00,0x00,0x00,0x00
        };
        uint8_t aad[32], pt[32];
        for (int i = 0; i < 32; i++) { aad[i] = (uint8_t)i; pt[i] = (uint8_t)i; }

        uint8_t ct[32]; uint8_t tag[16];
        SM4_KEY sk;
        sm4_set_encrypt_key(&sk, key);
        int ret = sm4_gcm_encrypt(&sk, iv, 12, aad, 32, pt, 32, ct, 16, tag);
        printf("SM4-GCM ret=%d\n", ret);
        hexdump("  CT  = ", ct, 32);
        hexdump("  TAG = ", tag, 16);

        uint8_t out[32]; memset(out, 0, 32);
        ret = sm4_gcm_decrypt(&sk, iv, 12, aad, 32, ct, 32, tag, 16, out);
        printf("  decrypt ret=%d, match=%d\n", ret, memcmp(out, pt, 32) == 0);
    }

    return 0;
}
