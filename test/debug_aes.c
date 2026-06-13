#include <stdio.h>
#include <string.h>
#include <stdint.h>
#include "../include/aes.h"

static void print_hex(const char *label, const uint8_t *buf, size_t len) {
    size_t i;
    printf("%s", label);
    for (i = 0; i < len; i++) printf("%02x", buf[i]);
    printf("\n");
}

int main(void) {
    /* FIPS 197 Appendix B: key=00010203...0f, in=00112233...ff, out=69c4e0d86a7b0430d8cdb78070b4c55a */
    /* Also use the simple B example: key=2b7e1516..., in=3243f6a8... */
    uint8_t key[16] = {
        0x2b,0x7e,0x15,0x16, 0x28,0xae,0xd2,0xa6,
        0xab,0xf7,0x15,0x88, 0x09,0xcf,0x4f,0x3c
    };
    uint8_t pt[16] = {
        0x32,0x43,0xf6,0xa8, 0x88,0x5a,0x30,0x8d,
        0x31,0x31,0x98,0xa2, 0xe0,0x37,0x07,0x34
    };
    uint8_t expected_ct[16] = {
        0x39,0x25,0x84,0x1d, 0x02,0xdc,0x09,0xfb,
        0xdc,0x11,0x85,0x97, 0x19,0x6a,0x0b,0x32
    };
    uint8_t ct[16];
    uint8_t pt2[16];
    AES_KEY ek, dk;

    aes_set_encrypt_key(&ek, key, 16);
    aes_encrypt(&ek, pt, ct);
    print_hex("pt       = ", pt, 16);
    print_hex("expected = ", expected_ct, 16);
    print_hex("actual   = ", ct, 16);

    aes_set_decrypt_key(&dk, key, 16);
    aes_decrypt(&dk, ct, pt2);
    print_hex("decrypt  = ", pt2, 16);
    print_hex("expected = ", pt, 16);

    /* Now round-key inspection */
    printf("\nRound keys (ek):\n");
    for (int i = 0; i < 11; i++) {
        printf("  rk[%d] = ", i);
        for (int j = 0; j < 4; j++) printf("%08x ", ek.rk[i*4 + j]);
        printf("\n");
    }
    printf("Round keys (dk):\n");
    for (int i = 0; i < 11; i++) {
        printf("  rk[%d] = ", i);
        for (int j = 0; j < 4; j++) printf("%08x ", dk.rk[i*4 + j]);
        printf("\n");
    }

    return 0;
}
