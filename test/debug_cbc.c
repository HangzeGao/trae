#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include "../include/aes.h"

static void print_hex(const char *label, const uint8_t *buf, size_t len) {
    printf("%s", label);
    for (size_t i = 0; i < len; i++) printf("%02x", buf[i]);
    printf("\n");
}

int main(void) {
    uint8_t key[16] = {
        0x2b,0x7e,0x15,0x16, 0x28,0xae,0xd2,0xa6,
        0xab,0xf7,0x15,0x88, 0x09,0xcf,0x4f,0x3c
    };
    uint8_t iv[16] = {
        0x00,0x01,0x02,0x03, 0x04,0x05,0x06,0x07,
        0x08,0x09,0x0a,0x0b, 0x0c,0x0d,0x0e,0x0f
    };
    uint8_t pt[64] = {
        0x6b,0xc1,0xbe,0xe2, 0x2e,0x40,0x9f,0x96,
        0xe9,0x3d,0x7e,0x11, 0x73,0x93,0x17,0x2a,
        0xae,0x2d,0x8a,0x57, 0x1e,0x03,0xac,0x9c,
        0x9e,0xb7,0x6f,0xac, 0x45,0xaf,0x8e,0x51,
        0x30,0xc8,0x1c,0x46, 0xa3,0x5c,0xe4,0x11,
        0xe5,0xfb,0xc1,0x19, 0x1a,0x0a,0x52,0xef,
        0xf6,0x9f,0x24,0x45, 0xdf,0x4f,0x9b,0x17,
        0xad,0x2b,0x41,0x7b, 0xe6,0x6c,0x37,0x10,
    };
    uint8_t ct[64];
    uint8_t buf[64];
    AES_KEY ek, dk;

    /* Encrypt in-place simulation */
    memcpy(buf, pt, 64);
    aes_set_encrypt_key(&ek, key, 16);
    memcpy(ct, iv, 16);
    aes_cbc_encrypt_blocks(&ek, iv, pt, 4, ct);
    /* Note: ct first 16 bytes was iv; but we want 4 blocks of ciphertext
       Actually the function does 4 blocks and overwrites ct[0:63] with ciphertext */
    print_hex("ct = ", ct, 64);

    /* Decrypt with overlapping buffers (like test_crypto does) */
    aes_set_decrypt_key(&dk, key, 16);
    memcpy(buf, ct, 64);
    memcpy(ct, iv, 16); /* reset iv */
    aes_cbc_decrypt_blocks(&dk, iv, buf, 4, buf);
    print_hex("decrypted = ", buf, 64);
    print_hex("expected  = ", pt, 64);

    /* Step by step debug */
    printf("\n=== Step-by-step ===\n");
    memcpy(buf, ct, 64); /* ct = ciphertext (already overwritten above, redo) */
    /* Actually let's recompute ct cleanly */
    uint8_t ct2[64];
    memcpy(ct2, iv, 16);
    aes_cbc_encrypt_blocks(&ek, iv, pt, 4, ct2);
    memcpy(buf, ct2, 64);

    printf("buf before decrypt (ct) = ");
    for (int i = 0; i < 4; i++) {
        for (int j = 0; j < 16; j++) printf("%02x", buf[i*16+j]);
        printf(" ");
    }
    printf("\n");

    return 0;
}
