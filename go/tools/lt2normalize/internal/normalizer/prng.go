package normalizer

const (
	generatedPRNGMultiplier        = uint32(0x01DF5E0D)
	generatedPRNGMultiplierInverse = uint32(0x01ABFCC5)
	generatedPRNGModulus           = uint32(100000000)
)

func generatedPRNGNext(state *uint32) uint32 {
	value := (generatedPRNGMulMod100M(*state, generatedPRNGMultiplier) + 1) % generatedPRNGModulus
	*state = value
	return ((value / 10000) << 8) / 10000
}

func generatedPRNGDword(state *uint32) uint32 {
	b0 := generatedPRNGNext(state) & 0xFF
	b1 := generatedPRNGNext(state) & 0xFF
	b2 := generatedPRNGNext(state) & 0xFF
	b3 := generatedPRNGNext(state) & 0xFF
	return b3 | (b0 << 24) | (b1 << 16) | (b2 << 8)
}

func generatedPRNGMulMod100M(a uint32, b uint32) uint32 {
	const chunk = uint32(10000)
	return ((((b/chunk)*(a%chunk))+((a/chunk)*(b%chunk)))%chunk*chunk +
		((b % chunk) * (a % chunk))) % generatedPRNGModulus
}

func generatedPRNGNextSigned(state *uint32) uint32 {
	value := (generatedPRNGMulMod100MSigned(*state, generatedPRNGMultiplier) + 1) % generatedPRNGModulus
	*state = value
	return ((value / 10000) << 8) / 10000
}

func generatedPRNGDwordSigned(state *uint32) uint32 {
	b0 := generatedPRNGNextSigned(state) & 0xFF
	b1 := generatedPRNGNextSigned(state) & 0xFF
	b2 := generatedPRNGNextSigned(state) & 0xFF
	b3 := generatedPRNGNextSigned(state) & 0xFF
	return b3 | (b0 << 24) | (b1 << 16) | (b2 << 8)
}

func generatedPRNGMulMod100MSigned(a uint32, b uint32) uint32 {
	q1, r1 := idivQR32(a, 10000)
	q2, r2 := idivQR32(b, 10000)
	high := (uint32(int64(q2)*int64(r1)) + uint32(int64(q1)*int64(r2))) % 10000
	low := uint32(int64(r2) * int64(r1))
	return (high*10000 + low) % generatedPRNGModulus
}

func idivQR32(value uint32, divisor int32) (int32, int32) {
	quotient := int32(value) / divisor
	remainder := int32(value) % divisor
	return quotient, remainder
}
