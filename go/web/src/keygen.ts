const shortV3Alphabet = "0123456789ABCDEFGHJKMNPQRTUVWXYZ";
const format = "Armadillo ShortV3 signed key";
const shortV3Level = 25;
const shortV3LevelIndex = shortV3Level - 20;
const shortV3CertSeed = 0xccf0580a;
const shortV3Private = 0x70301169de7c75d66fn;
const shortV3PrimeBase = 0xf3c7e00a4b58155299n;
const primeOffsets = [15, 15, 21, 81, 3103, 3643, 2191, 9691, 2887] as const;
const textEncoder = new TextEncoder();

const md5Shift = [
  7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
  5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
  4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
  6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21
] as const;

const md5K = Array.from({ length: 64 }, (_, i) => Math.floor(Math.abs(Math.sin(i + 1)) * 2 ** 32) >>> 0);
const aktCRC32Table = buildAKTCRC32Table();

export type KeygenPair = {
  registrationName: string;
  activationKey: string;
  format: string;
};

export function generateActivationPair(requestedName: string): KeygenPair {
  const registrationName = requestedName.trim() || randomName(12);
  return {
    registrationName,
    activationKey: generateSignedKey(registrationName, currentArmadilloDate(), randomSignatureSeed),
    format
  };
}

export function generateSignedKey(name: string, today: number, seedSource: () => number): string {
  if (cookText(name) === "") {
    throw new Error("registration name cannot be empty");
  }

  const keyBytes: number[] = [];
  appendUint16LE(keyBytes, today);
  appendUint32LE(keyBytes, reverseBytes32(shortV3CertSeed));

  const rng = newAKTRandom(aktCRC32(textEncoder.encode(cookText(name)), 0xffffffff));
  for (let i = 0; i < keyBytes.length; i += 1) {
    keyBytes[i] ^= rng.nextRange(256);
  }

  const size = shortV3LevelIndex + 4;
  const p = (1n << BigInt(size * 8)) + BigInt(primeOffsets[shortV3LevelIndex]);
  const pMinusOne = p - 1n;
  const message = shortV3Message(keyBytes, name);
  let [a, b] = makeElGamalSignature(message, shortV3Private, shortV3PrimeBase, p, pMinusOne, size, seedSource);

  for (let i = 0; i < size; i += 1) {
    keyBytes.push(Number(a & 0xffn));
    a >>= 8n;
    keyBytes.push(Number(b & 0xffn));
    b >>= 8n;
  }

  return encodeShortV3Key(keyBytes);
}

function randomName(length: number): string {
  const values = new Uint8Array(length);
  crypto.getRandomValues(values);
  let result = "";
  for (const value of values) {
    result += shortV3Alphabet[value % shortV3Alphabet.length];
  }
  return result;
}

function currentArmadilloDate(): number {
  const epoch = Date.UTC(1999, 0, 1);
  return Math.floor((Date.now() - epoch) / 86_400_000) & 0xffff;
}

function randomSignatureSeed(): number {
  const values = new Uint32Array(1);
  crypto.getRandomValues(values);
  return values[0] || 1000;
}

function appendUint16LE(out: number[], value: number) {
  out.push(value & 0xff, (value >>> 8) & 0xff);
}

function appendUint32LE(out: number[], value: number) {
  out.push(value & 0xff, (value >>> 8) & 0xff, (value >>> 16) & 0xff, (value >>> 24) & 0xff);
}

function reverseBytes32(value: number): number {
  return (((value & 0xff) << 24) | ((value & 0xff00) << 8) | ((value >>> 8) & 0xff00) | (value >>> 24)) >>> 0;
}

function newAKTRandom(seed: number) {
  let state = seed >>> 0;
  return {
    nextRange(n: number) {
      state = (aktMult(state, 31_415_821) + 1) % 100_000_000;
      return Math.floor((Math.floor(state / 10_000) * n) / 10_000) & 0xff;
    }
  };
}

function aktMult(p: number, q: number): number {
  const m1 = 10_000;
  const p1 = Math.floor(p / m1);
  const p0 = p % m1;
  const q1 = Math.floor(q / m1);
  const q0 = q % m1;
  return ((((p0 * q1 + p1 * q0) % m1) * m1 + p0 * q0) % 100_000_000) >>> 0;
}

function cookText(text: string): string {
  let result = "";
  for (let i = 0; i < text.length; i += 1) {
    let code = text.charCodeAt(i);
    if (code === 0x20 || code === 0x09 || code === 0x0d || code === 0x0a) continue;
    if (code >= 0x61 && code <= 0x7a) code -= 0x20;
    result += String.fromCharCode(code);
  }
  return result;
}

function buildAKTCRC32Table(): Uint32Array {
  const table = new Uint32Array(256);
  const poly32 = 0x04c11db7;
  for (let x = 0; x < table.length; x += 1) {
    let r = reflectBits(x, 8) << 24;
    for (let i = 0; i < 8; i += 1) {
      r = r & 0x80000000 ? ((r << 1) ^ poly32) >>> 0 : (r << 1) >>> 0;
    }
    table[x] = reflectBits(r, 32);
  }
  return table;
}

function reflectBits(value: number, bitCount: number): number {
  let out = 0;
  for (let i = 0; i < bitCount; i += 1) {
    if (value & (1 << i)) out |= 1 << (bitCount - 1 - i);
  }
  return out >>> 0;
}

function aktCRC32(data: Uint8Array, crc: number): number {
  let result = crc >>> 0;
  for (const byte of data) {
    result = (aktCRC32Table[(result ^ byte) & 0xff] ^ (result >>> 8)) >>> 0;
  }
  return result;
}

function getKeyCRC(text: string, period: number): number {
  let cooked = cookText(text);
  if (period === 1) {
    cooked = reverseString(cooked);
  } else if (period > 1) {
    let grouped = "";
    for (let i = 0; i < cooked.length; i += period) {
      const end = Math.min(i + period, cooked.length);
      for (let j = end - 1; j >= i; j -= 1) grouped += cooked[j];
    }
    cooked = grouped;
  }
  return aktCRC32(textEncoder.encode(cooked), 0xffffffff);
}

function reverseString(text: string): string {
  return [...text].reverse().join("");
}

function shortV3Message(keyBytes: number[], name: string): bigint {
  const cookedName = textEncoder.encode(cookText(name));
  const payload = new Uint8Array(keyBytes.length + cookedName.length);
  payload.set(keyBytes);
  payload.set(cookedName, keyBytes.length);
  const sum = md5(payload);
  let message = 0n;
  for (let i = 0; i < sum.length; i += 4) {
    message = (message << 32n) + BigInt(readUint32LE(sum, i));
  }
  return message;
}

function makeElGamalSignature(
  message: bigint,
  privateKey: bigint,
  pubBase: bigint,
  p: bigint,
  pMinusOne: bigint,
  size: number,
  seedSource: () => number
): [bigint, bigint] {
  const minPart = 256n;
  const maxPart = 1n << BigInt(size * 8);
  let seed = seedSource() >>> 0;
  if (seed === 0) seed = 1000;

  for (let attempts = 0; attempts < 1_000_000; attempts += 1) {
    const kSource = formatHex8((seed + attempts) >>> 0);
    let k = 0n;
    for (let x = 0; x < 5; x += 1) {
      k = (k << 4n) + BigInt(getKeyCRC(kSource, x + 2));
    }
    k = mod(k, p);
    if (k <= 0n || gcd(k, pMinusOne) !== 1n) continue;

    const a = powMod(pubBase, k, p);
    const kInv = modInverse(k, pMinusOne);
    if (kInv === null) continue;
    const b = mod((message - privateKey * a) * kInv, pMinusOne);

    if (a >= minPart && a < maxPart && b >= minPart && b < maxPart) return [a, b];
  }

  throw new Error("failed to produce bounded signature");
}

function formatHex8(value: number): string {
  return (value >>> 0).toString(16).toUpperCase().padStart(8, "0");
}

function encodeShortV3Key(keyBytes: number[]): string {
  let n = bigIntFromBytes(keyBytes);
  const out: string[] = [];
  let dcount = 6;
  while (n !== 0n) {
    const digit = Number(n % 32n);
    n >>= 5n;
    if (n === 0n) {
      if (digit < 16) {
        out.push(shortV3Alphabet[digit + 16]);
        dcount -= 1;
      } else {
        out.push(shortV3Alphabet[digit]);
        dcount -= 1;
        if (dcount === 0) {
          out.push("-");
          dcount = 6;
        }
        out.push(shortV3Alphabet[16]);
        dcount -= 1;
      }
    } else {
      out.push(shortV3Alphabet[digit]);
      dcount -= 1;
      if (dcount === 0) {
        out.push("-");
        dcount = 6;
      }
    }
  }
  while (dcount > 0) {
    out.push("0");
    dcount -= 1;
  }
  return out.reverse().join("");
}

function bigIntFromBytes(bytes: number[]): bigint {
  let result = 0n;
  for (const byte of bytes) {
    result = (result << 8n) + BigInt(byte);
  }
  return result;
}

function powMod(base: bigint, exponent: bigint, modulus: bigint): bigint {
  let result = 1n;
  let b = mod(base, modulus);
  let e = exponent;
  while (e > 0n) {
    if (e & 1n) result = mod(result * b, modulus);
    b = mod(b * b, modulus);
    e >>= 1n;
  }
  return result;
}

function gcd(left: bigint, right: bigint): bigint {
  let a = left < 0n ? -left : left;
  let b = right < 0n ? -right : right;
  while (b !== 0n) {
    const next = a % b;
    a = b;
    b = next;
  }
  return a;
}

function modInverse(value: bigint, modulus: bigint): bigint | null {
  let t = 0n;
  let newT = 1n;
  let r = modulus;
  let newR = mod(value, modulus);
  while (newR !== 0n) {
    const quotient = r / newR;
    [t, newT] = [newT, t - quotient * newT];
    [r, newR] = [newR, r - quotient * newR];
  }
  if (r !== 1n) return null;
  return mod(t, modulus);
}

function mod(value: bigint, modulus: bigint): bigint {
  const result = value % modulus;
  return result < 0n ? result + modulus : result;
}

function md5(input: Uint8Array): Uint8Array {
  const bitLength = input.length * 8;
  let paddedLength = input.length + 1;
  while (paddedLength % 64 !== 56) paddedLength += 1;

  const message = new Uint8Array(paddedLength + 8);
  message.set(input);
  message[input.length] = 0x80;
  for (let i = 0; i < 8; i += 1) {
    message[paddedLength + i] = Math.floor(bitLength / 2 ** (8 * i)) & 0xff;
  }

  let h0 = 0x67452301;
  let h1 = 0xefcdab89;
  let h2 = 0x98badcfe;
  let h3 = 0x10325476;

  const words = new Uint32Array(16);
  for (let offset = 0; offset < message.length; offset += 64) {
    for (let i = 0; i < 16; i += 1) words[i] = readUint32LE(message, offset + i * 4);

    let a = h0;
    let b = h1;
    let c = h2;
    let d = h3;

    for (let i = 0; i < 64; i += 1) {
      let f: number;
      let g: number;
      if (i < 16) {
        f = (b & c) | (~b & d);
        g = i;
      } else if (i < 32) {
        f = (d & b) | (~d & c);
        g = (5 * i + 1) % 16;
      } else if (i < 48) {
        f = b ^ c ^ d;
        g = (3 * i + 5) % 16;
      } else {
        f = c ^ (b | ~d);
        g = (7 * i) % 16;
      }

      const previousD = d;
      d = c;
      c = b;
      b = (b + leftRotate((a + f + md5K[i] + words[g]) >>> 0, md5Shift[i])) >>> 0;
      a = previousD;
    }

    h0 = (h0 + a) >>> 0;
    h1 = (h1 + b) >>> 0;
    h2 = (h2 + c) >>> 0;
    h3 = (h3 + d) >>> 0;
  }

  const digest = new Uint8Array(16);
  writeUint32LE(digest, 0, h0);
  writeUint32LE(digest, 4, h1);
  writeUint32LE(digest, 8, h2);
  writeUint32LE(digest, 12, h3);
  return digest;
}

function leftRotate(value: number, count: number): number {
  return ((value << count) | (value >>> (32 - count))) >>> 0;
}

function readUint32LE(bytes: Uint8Array, offset: number): number {
  return (
    bytes[offset] |
    (bytes[offset + 1] << 8) |
    (bytes[offset + 2] << 16) |
    (bytes[offset + 3] << 24)
  ) >>> 0;
}

function writeUint32LE(bytes: Uint8Array, offset: number, value: number) {
  bytes[offset] = value & 0xff;
  bytes[offset + 1] = (value >>> 8) & 0xff;
  bytes[offset + 2] = (value >>> 16) & 0xff;
  bytes[offset + 3] = (value >>> 24) & 0xff;
}
