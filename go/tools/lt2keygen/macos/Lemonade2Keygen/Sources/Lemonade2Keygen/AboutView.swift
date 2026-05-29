import SwiftUI

enum AboutTopic: String, CaseIterable, Identifiable {
    case overview
    case name
    case payload
    case masking
    case hash
    case modulo
    case signature
    case encoding

    var id: String { rawValue }

    var title: String {
        switch self {
        case .overview: "Overview"
        case .name: "Name Binding"
        case .payload: "Payload"
        case .masking: "Masking"
        case .hash: "Hashing"
        case .modulo: "Modular Math"
        case .signature: "Signature"
        case .encoding: "Encoding"
        }
    }

    var systemImage: String {
        switch self {
        case .overview: "map"
        case .name: "person.text.rectangle"
        case .payload: "shippingbox"
        case .masking: "shuffle"
        case .hash: "number"
        case .modulo: "clock"
        case .signature: "signature"
        case .encoding: "textformat.123"
        }
    }
}

struct AboutView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var selectedTopic = AboutTopic.overview
    @State private var demoName = "Ada Lovelace"
    @State private var xorPayload = "3A"
    @State private var xorMask = "10"
    @State private var moduloValue = "17"
    @State private var moduloBase = "12"
    @State private var inverseValue = "5"
    @State private var inverseModulus = "12"

    var body: some View {
        HStack(spacing: 0) {
            sidebar

            Divider()

            VStack(spacing: 0) {
                header
                Divider()
                ScrollView {
                    detailContent(for: selectedTopic)
                        .padding(28)
                        .frame(maxWidth: 760, alignment: .leading)
                        .frame(maxWidth: .infinity, alignment: .center)
                }
            }
            .background(Color(nsColor: .windowBackgroundColor))
        }
        .frame(minWidth: 900, minHeight: 680)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button("Done") { dismiss() }
                    .keyboardShortcut(.cancelAction)
            }
        }
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("How It Works")
                .font(.headline)
                .padding(.bottom, 8)

            ForEach(AboutTopic.allCases) { topic in
                Button {
                    selectedTopic = topic
                } label: {
                    Label(topic.title, systemImage: topic.systemImage)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.vertical, 6)
                        .padding(.horizontal, 8)
                        .background(selectedTopic == topic ? Color.accentColor.opacity(0.16) : Color.clear, in: RoundedRectangle(cornerRadius: 8, style: .continuous))
                }
                .buttonStyle(.plain)
                .foregroundStyle(selectedTopic == topic ? Color.accentColor : Color.primary)
            }

            Spacer()
        }
        .padding(16)
        .frame(width: 220)
        .background(.thinMaterial)
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: 5) {
            Text(selectedTopic.title)
                .font(.title2.bold())
            Text("Lemonade Tycoon 2: New York Edition uses a signed Armadillo ShortV3 key. This walkthrough explains each step with small examples.")
                .font(.callout)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.horizontal, 28)
        .padding(.vertical, 18)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.bar)
    }

    @ViewBuilder
    private func detailContent(for topic: AboutTopic) -> some View {
        switch topic {
        case .overview: overviewSection
        case .name: nameSection
        case .payload: payloadSection
        case .masking: maskingSection
        case .hash: hashSection
        case .modulo: moduloSection
        case .signature: signatureSection
        case .encoding: encodingSection
        }
    }

    private var overviewSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("The big picture")
                .font(.largeTitle.bold())
            Text("A generated key is a small license payload plus a digital signature. The payload says what the key is for; the signature proves the payload and registration name match what the game expects.")
                .font(.title3)
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            FormulaBlock(lines: [
                "1. Normalize the registration name",
                "2. Build a tiny payload: date + certificate seed",
                "3. Mask that payload with a name-seeded byte stream",
                "4. Hash masked payload + normalized name into m",
                "5. Sign m with Armadillo ShortV3 ElGamal math",
                "6. Encode the bytes using Armadillo's ShortV3 alphabet",
            ])

            Text("Use the sidebar to walk through the steps. Each section introduces only the math needed for that step.")
                .foregroundStyle(.secondary)
        }
    }

    private var nameSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("The name is part of the math")
                .font(.largeTitle.bold())
            Text("The activation key is not checked alone. The game checks the registration name and the activation key together. That is why a key generated for one name fails with another name.")
                .fixedSize(horizontal: false, vertical: true)
            nameDemo
            ExampleBlock(title: "Why changing the name breaks it", body: "The normalized name is used both when masking the payload and when building the signed hash. If the name changes, those bytes change, and the old signature no longer matches.")
        }
    }

    private var payloadSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("The payload is only six bytes before signing")
                .font(.largeTitle.bold())
            Text("A payload is the small data packet carried by the key. For this game, the payload starts with an Armadillo date and a recovered certificate seed from the registration path.")
                .fixedSize(horizontal: false, vertical: true)
            FormulaBlock(lines: [
                "date = days since 1999-01-01       // 16 bits = 2 bytes",
                "certificate seed = 0xCCF0580A      // 32 bits = 4 bytes",
                "payload = date || byteSwap(seed)   // 6 bytes total",
            ])
            Text("A bit is a single 0-or-1 value. Eight bits make one byte. Byte-swapping reverses byte order so the value is stored the way the target's verifier reads it.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var maskingSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("The payload is masked with a name-based stream")
                .font(.largeTitle.bold())
            Text("CRC32 turns the normalized name into a 32-bit starting value. Armadillo's pseudo-random generator expands that seed into bytes. Each payload byte is XORed with one random byte.")
                .fixedSize(horizontal: false, vertical: true)
            FormulaBlock(lines: [
                "seed = CRC32(normalizedName, 0xFFFFFFFF)",
                "state = (state * 31415821 + 1) mod 100000000",
                "maskedByte = payloadByte XOR randomByte",
            ])
            xorDemo
            Text("XOR is reversible: applying the same random byte again recovers the original byte. This is scrambling, not modern encryption; it reproduces the Armadillo verifier format.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var hashSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Hashing turns bytes into the message number")
                .font(.largeTitle.bold())
            Text("The signature signs a number. The generator creates that number by hashing the masked payload and normalized name with MD5, then reading the digest as little-endian 32-bit chunks.")
                .fixedSize(horizontal: false, vertical: true)
            FormulaBlock(lines: [
                "bytesToSign = maskedPayload || normalizedName",
                "digest = MD5(bytesToSign)       // always 16 bytes",
                "m = digest interpreted as a big integer",
            ])
            Text("MD5 is not chosen as modern security here. It is part of the legacy ShortV3 format that the game already expects.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var moduloSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Modulo means wrap-around math")
                .font(.largeTitle.bold())
            Text("Instead of allowing numbers to grow forever, modular arithmetic wraps every result around a modulus. Clock arithmetic is the everyday version: 17 mod 12 is 5.")
                .fixedSize(horizontal: false, vertical: true)
            moduloDemo
            FormulaBlock(lines: [
                "ShortV3 level = 25",
                "field size = 9 bytes = 72 bits",
                "p = 2^72 + 3643",
            ])
            Text("The real modulus p is huge compared with the demo. The idea is identical: multiply, exponentiate, and wrap around p.")
                .foregroundStyle(.secondary)
        }
    }

    private var signatureSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("ElGamal signing creates the verifier pair")
                .font(.largeTitle.bold())
            Text("The game has public verification data. The generator has the matching recovered private exponent. The private exponent creates a signature; the public data lets the game verify it.")
                .fixedSize(horizontal: false, vertical: true)
            FormulaBlock(lines: [
                "public y = g^x mod p",
                "a = g^k mod p",
                "b = (m - x*a) * k^-1 mod (p - 1)",
            ])
            inverseDemo
            Text("k is a per-key nonce. k^-1 is the modular inverse of k. The inverse exists only when gcd(k, p-1) = 1, so the generator rejects nonce values that do not satisfy that condition.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            toySignatureDemo
        }
    }

    private var encodingSection: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Encoding makes the bytes typable")
                .font(.largeTitle.bold())
            Text("After signing, the generator appends the signature bytes to the masked payload, interleaving a and b as a0, b0, a1, b1, and so on. It then converts the byte string into Armadillo's ShortV3 alphabet.")
                .fixedSize(horizontal: false, vertical: true)
            FormulaBlock(lines: [
                "alphabet = 0123456789ABCDEFGHJKMNPQRTUVWXYZ",
                "grouping = hyphen every 6 characters",
            ])
            Text("The alphabet omits ambiguous letters such as I, L, O, and S so the code is easier to read and type into the game.")
                .foregroundStyle(.secondary)
        }
    }

    private var nameDemo: some View {
        InteractiveCard(title: "Try normalization", caption: "Spaces are removed and letters are uppercased before hashing.") {
            TextField("Name", text: $demoName)
                .textFieldStyle(.roundedBorder)
            FormulaBlock(lines: ["normalized = \(normalizeName(demoName))"])
        }
    }

    private var xorDemo: some View {
        let payload = parseHexByte(xorPayload)
        let mask = parseHexByte(xorMask)
        let result = payload.flatMap { p in mask.map { p ^ $0 } }
        return InteractiveCard(title: "Try XOR", caption: "Enter two hex bytes and see the masked byte.") {
            HStack {
                TextField("payload", text: $xorPayload)
                    .textFieldStyle(.roundedBorder)
                Text("XOR")
                TextField("mask", text: $xorMask)
                    .textFieldStyle(.roundedBorder)
            }
            FormulaBlock(lines: ["result = \(result.map(hexByte) ?? "invalid hex")"])
        }
    }

    private var moduloDemo: some View {
        let value = parseInt(moduloValue)
        let base = parseInt(moduloBase)
        let result = value.flatMap { v in base.flatMap { modulo(v, $0) } }
        return InteractiveCard(title: "Try modulo", caption: "Use small numbers to see wrap-around behavior.") {
            HStack {
                TextField("value", text: $moduloValue)
                    .textFieldStyle(.roundedBorder)
                Text("mod")
                TextField("base", text: $moduloBase)
                    .textFieldStyle(.roundedBorder)
            }
            FormulaBlock(lines: ["remainder = \(result.map(String.init) ?? "invalid")"])
        }
    }

    private var inverseDemo: some View {
        let value = parseInt(inverseValue)
        let base = parseInt(inverseModulus)
        let inv = value.flatMap { v in base.flatMap { modularInverse(v, modulus: $0) } }
        return InteractiveCard(title: "Try modular inverse", caption: "Find a number that makes k * inverse wrap to 1.") {
            HStack {
                TextField("k", text: $inverseValue)
                    .textFieldStyle(.roundedBorder)
                Text("mod")
                TextField("modulus", text: $inverseModulus)
                    .textFieldStyle(.roundedBorder)
            }
            FormulaBlock(lines: ["inverse = \(inv.map(String.init) ?? "none")"])
        }
    }

    private var toySignatureDemo: some View {
        let toy = toySignature()
        return InteractiveCard(title: "Toy signature", caption: "Same structure as the real signature, using tiny numbers.") {
            if let toy {
                FormulaBlock(lines: [
                    "p=\(toy.p), g=\(toy.g), private x=\(toy.x), message m=\(toy.m), nonce k=\(toy.k)",
                    "public y = g^x mod p = \(toy.y)",
                    "a = g^k mod p = \(toy.a)",
                    "k^-1 mod (p-1) = \(toy.kInverse)",
                    "b = (m - x*a) * k^-1 mod (p-1) = \(toy.b)",
                    "verify: g^m mod p = \(toy.left)",
                    "verify: y^a * a^b mod p = \(toy.right)",
                ])
            }
        }
    }
}
