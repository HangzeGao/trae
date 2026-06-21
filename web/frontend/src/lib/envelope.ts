import type { ParsedEnvelope } from "./types";

// Envelope v1 fixed header is 61 bytes. Layout per design §8.3:
//   0-3:   magic "KVLT"
//   4:     version (1)
//   5-6:   flags (uint16 BE)
//   7-8:   suite_id (uint16 BE)
//   9-10:  key_id_len (uint16 BE)
//   11-14: key_version (uint32 BE)
//   15-18: policy_version (uint32 BE)
//   19:    nonce_len (uint8)
//   20:    tag_len (uint8)
//   21-28: ciphertext_len (uint64 BE)
//   29-60: aad_hash (32 bytes SHA-256)
const FIXED_HEADER_SIZE = 61;

const SUITE_NAMES: Record<number, string> = {
  0x0001: "AES_256_GCM",
  0x0002: "SM4_GCM",
  0x0003: "AES_128_GCM",
  0x0101: "AES_256_CBC_HMAC_SHA256",
};

function toHex(bytes: Uint8Array): string {
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export function parseEnvelope(base64Str: string): ParsedEnvelope | null {
  let raw: Uint8Array;
  try {
    const bin = atob(base64Str.trim());
    raw = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) raw[i] = bin.charCodeAt(i);
  } catch {
    return null;
  }

  if (raw.length < FIXED_HEADER_SIZE) return null;

  const magic = String.fromCharCode(raw[0], raw[1], raw[2], raw[3]);
  if (magic !== "KVLT") return null;

  const version = raw[4];
  if (version !== 1) return null;

  const dv = new DataView(raw.buffer, raw.byteOffset, raw.byteLength);
  const flags = dv.getUint16(5);
  const suiteNum = dv.getUint16(7);
  const keyIdLen = dv.getUint16(9);
  const keyVersion = dv.getUint32(11);
  const policyVersion = dv.getUint32(15);
  const nonceLen = raw[19];
  const tagLen = raw[20];
  const ciphertextLen = Number(dv.getBigUint64(21));

  const aadHashEnd = FIXED_HEADER_SIZE;
  const keyIdStart = aadHashEnd;
  const keyId = new TextDecoder().decode(raw.slice(keyIdStart, keyIdStart + keyIdLen));

  const nonceStart = keyIdStart + keyIdLen;
  const nonce = raw.slice(nonceStart, nonceStart + nonceLen);

  const ctStart = nonceStart + nonceLen;
  const ciphertext = raw.slice(ctStart, ctStart + ciphertextLen);

  const tagStart = ctStart + ciphertextLen;
  const tag = raw.slice(tagStart, tagStart + tagLen);

  const aadHash = raw.slice(29, aadHashEnd);

  return {
    magic,
    version,
    flags,
    suite_id: SUITE_NAMES[suiteNum] ?? `0x${suiteNum.toString(16).padStart(4, "0")}`,
    key_id: keyId,
    key_version: keyVersion,
    policy_version: policyVersion,
    nonce_len: nonceLen,
    tag_len: tagLen,
    ciphertext_len: ciphertextLen,
    aad_hash: toHex(aadHash),
    nonce: toHex(nonce),
    ciphertext: toHex(ciphertext),
    tag: toHex(tag),
  };
}

export function toBase64(str: string): string {
  return btoa(str);
}

export function fromBase64(b64: string): string {
  try {
    return atob(b64);
  } catch {
    return "";
  }
}

export function bytesToHex(b64: string): string {
  try {
    const bin = atob(b64);
    let hex = "";
    for (let i = 0; i < bin.length; i++) {
      hex += bin.charCodeAt(i).toString(16).padStart(2, "0");
    }
    return hex;
  } catch {
    return "";
  }
}
