#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "aes.h"

int main() {
    uint8_t key[16] = {
        0x2b,0x7e,0x15,0x16,0x28,0xae,0xd2,0xa6,
        0xab,0xf7,0x15,0x88,0x09,0xcf,0x4f,0x3c
    };
    uint8_t pt[16] = {
        0x32,0x43,0xf6,0xa8,0x88,0x5a,0x30,0x8d,
        0x31,0x31,0x98,0xa2,0xe0,0x37,0x07,0x34
    };
    uint8_t ct[16], pt2[16];
    AES_KEY ek, dk;
    int i;

    aes_set_encrypt_key(&ek, key, 16);
    aes_encrypt(&ek, pt, ct);
    printf("encrypt: ");
    for (i=0;i<16;i++) printf("%02x", ct[i]);
    printf("\n");

    aes_set_decrypt_key(&dk, key, 16);
    aes_decrypt(&dk, ct, pt2);
    printf("decrypt: ");
    for (i=0;i<16;i++) printf("%02x", pt2[i]);
    printf("\n");
    return 0;
}
