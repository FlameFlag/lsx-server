import Foundation

func normalizeName(_ name: String) -> String {
    name.compactMap { character -> Character? in
        if character.isWhitespace { return nil }
        return Character(character.uppercased())
    }.map(String.init).joined()
}

func parseHexByte(_ text: String) -> UInt8? {
    let cleaned = text.trimmingCharacters(in: .whitespacesAndNewlines)
        .replacingOccurrences(of: "0x", with: "")
        .replacingOccurrences(of: "0X", with: "")
    guard let value = UInt8(cleaned, radix: 16) else { return nil }
    return value
}

func hexByte(_ value: UInt8) -> String {
    String(format: "0x%02X", value)
}

func parseInt(_ text: String) -> Int? {
    Int(text.trimmingCharacters(in: .whitespacesAndNewlines))
}

func modulo(_ value: Int, _ modulus: Int) -> Int? {
    guard modulus > 0 else { return nil }
    let result = value % modulus
    return result >= 0 ? result : result + modulus
}

func gcd(_ a: Int, _ b: Int) -> Int {
    var x = abs(a)
    var y = abs(b)
    while y != 0 {
        let r = x % y
        x = y
        y = r
    }
    return x
}

func modularInverse(_ value: Int, modulus: Int) -> Int? {
    guard modulus > 1, gcd(value, modulus) == 1 else { return nil }
    let normalized = modulo(value, modulus) ?? value
    for candidate in 1..<modulus {
        if (normalized * candidate) % modulus == 1 {
            return candidate
        }
    }
    return nil
}

func powMod(_ base: Int, _ exponent: Int, _ modulus: Int) -> Int? {
    guard modulus > 1, exponent >= 0 else { return nil }
    var result = 1
    var b = modulo(base, modulus) ?? base
    var e = exponent
    while e > 0 {
        if e % 2 == 1 {
            result = (result * b) % modulus
        }
        b = (b * b) % modulus
        e /= 2
    }
    return result
}

struct ToySignature {
    let p: Int
    let g: Int
    let x: Int
    let m: Int
    let k: Int
    let y: Int
    let a: Int
    let b: Int
    let left: Int
    let right: Int
    let kInverse: Int
}

func toySignature(p: Int = 23, g: Int = 5, x: Int = 6, m: Int = 7, k: Int = 5) -> ToySignature? {
    guard let y = powMod(g, x, p),
          let a = powMod(g, k, p),
          let kInverse = modularInverse(k, modulus: p - 1),
          let rawB = modulo((m - x * a) * kInverse, p - 1),
          let left = powMod(g, m, p),
          let ya = powMod(y, a, p),
          let ab = powMod(a, rawB, p),
          let right = modulo(ya * ab, p) else {
        return nil
    }
    return ToySignature(p: p, g: g, x: x, m: m, k: k, y: y, a: a, b: rawB, left: left, right: right, kInverse: kInverse)
}
