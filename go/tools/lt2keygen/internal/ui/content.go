package ui

const appTitle = "Lemonade Tycoon 2: New York Edition Keygen"

type aboutTopic struct {
	Title string
	Body  string
}

var aboutTopics = []aboutTopic{
	{
		Title: "Overview",
		Body: "A generated key is a small license payload plus a digital signature. The payload says what the key is for; the signature proves the payload and registration name match what the game expects.\r\n\r\n" +
			"1. Normalize the registration name\r\n" +
			"2. Build a tiny payload: date + certificate seed\r\n" +
			"3. Mask that payload with a name-seeded byte stream\r\n" +
			"4. Hash masked payload + normalized name into m\r\n" +
			"5. Sign m with Armadillo ShortV3 ElGamal math\r\n" +
			"6. Encode the bytes using Armadillo's ShortV3 alphabet",
	},
	{
		Title: "Name Binding",
		Body: "The activation key is not checked alone. The game checks the registration name and the activation key together. That is why a key generated for one name fails with another name.\r\n\r\n" +
			"Before hashing, spaces are removed and ASCII lowercase letters are uppercased. The normalized name is used both when masking the payload and when building the signed hash. If the name changes, those bytes change, and the old signature no longer matches.",
	},
	{
		Title: "Payload",
		Body: "A payload is the small data packet carried by the key. For this game, the payload starts with an Armadillo date and a recovered certificate seed from the registration path.\r\n\r\n" +
			"date = days since 1999-01-01       // 16 bits = 2 bytes\r\n" +
			"certificate seed = 0xCCF0580A      // 32 bits = 4 bytes\r\n" +
			"payload = date || byteSwap(seed)   // 6 bytes total\r\n\r\n" +
			"A bit is a single 0-or-1 value. Eight bits make one byte. Byte-swapping reverses byte order so the value is stored the way the target verifier reads it.",
	},
	{
		Title: "Masking",
		Body: "CRC32 turns the normalized name into a 32-bit starting value. Armadillo's pseudo-random generator expands that seed into bytes. Each payload byte is XORed with one random byte.\r\n\r\n" +
			"seed = CRC32(normalizedName, 0xFFFFFFFF)\r\n" +
			"state = (state * 31415821 + 1) mod 100000000\r\n" +
			"maskedByte = payloadByte XOR randomByte\r\n\r\n" +
			"XOR is reversible: applying the same random byte again recovers the original byte. This is scrambling, not modern encryption; it reproduces the Armadillo verifier format.",
	},
	{
		Title: "Hashing",
		Body: "The signature signs a number. The generator creates that number by hashing the masked payload and normalized name with MD5, then reading the digest as little-endian 32-bit chunks.\r\n\r\n" +
			"bytesToSign = maskedPayload || normalizedName\r\n" +
			"digest = MD5(bytesToSign)       // always 16 bytes\r\n" +
			"m = digest interpreted as a big integer\r\n\r\n" +
			"MD5 is not chosen as modern security here. It is part of the legacy ShortV3 format that the game already expects.",
	},
	{
		Title: "Modular Math",
		Body: "Instead of allowing numbers to grow forever, modular arithmetic wraps every result around a modulus. Clock arithmetic is the everyday version: 17 mod 12 is 5.\r\n\r\n" +
			"ShortV3 level = 25\r\n" +
			"field size = 9 bytes = 72 bits\r\n" +
			"p = 2^72 + 3643\r\n\r\n" +
			"The real modulus p is huge compared with clock arithmetic. The idea is identical: multiply, exponentiate, and wrap around p.",
	},
	{
		Title: "Signature",
		Body: "The game has public verification data. The generator has the matching recovered private exponent. The private exponent creates a signature; the public data lets the game verify it.\r\n\r\n" +
			"public y = g^x mod p\r\n" +
			"a = g^k mod p\r\n" +
			"b = (m - x*a) * k^-1 mod (p - 1)\r\n\r\n" +
			"k is a per-key nonce. k^-1 is the modular inverse of k. The inverse exists only when gcd(k, p-1) = 1, so the generator rejects nonce values that do not satisfy that condition.",
	},
	{
		Title: "Encoding",
		Body: "After signing, the generator appends the signature bytes to the masked payload, interleaving a and b as a0, b0, a1, b1, and so on. It then converts the byte string into Armadillo's ShortV3 alphabet.\r\n\r\n" +
			"alphabet = 0123456789ABCDEFGHJKMNPQRTUVWXYZ\r\n" +
			"grouping = hyphen every 6 characters\r\n\r\n" +
			"The alphabet omits ambiguous letters such as I, L, O, and S so the code is easier to read and type into the game.",
	},
}
