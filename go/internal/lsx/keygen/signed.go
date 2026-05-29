package keygen

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"math/bits"
	"strings"
	"time"
)

const shortV3Alphabet = "0123456789ABCDEFGHJKMNPQRTUVWXYZ"

const Format = "Armadillo ShortV3 signed key"

const (
	shortV3Level      = 25
	shortV3LevelIndex = shortV3Level - 20

	// Recovered from the target's live Armadillo public/license table on the
	// REGISTER public-key path. The working record was kind 25, length 9,
	// check/id D074508D, with this public certificate value.
	shortV3PublicCertHex = "9CC50E4D25416464B9"

	// Matching core license seed for check/id D074508D. The key payload stores
	// this little-endian, but the target parses the decrypted dword as big-endian,
	// so generateSignedKey byte-swaps it before payload encryption.
	shortV3CertSeed = 0xccf0580a

	// Armadillo/AKT ShortV3 level-25 ElGamal parameters. shortV3PrimeBase is the
	// generator used by the verifier. shortV3PrivateHex is not embedded in the
	// target; it was recovered by solving the discrete log of shortV3PublicCertHex:
	//     shortV3PrimeBase^shortV3PrivateHex mod p == shortV3PublicCertHex
	// where p = 2^(9*8) + 0xE3B for level 25. signed_test.go verifies this, and
	// go run ./tools/shortv3derive prints the exact public inputs, p-1
	// factorization, verifier, and Sage/CADO-NFS reproduction commands.
	shortV3PrivateHex = "70301169DE7C75D66F"
	shortV3PrimeBase  = "F3C7E00A4B58155299"

	armadilloDateEpoch = "1999-01-01"
)

var primeOffsets = [...]int64{15, 15, 21, 81, 3103, 3643, 2191, 9691, 2887}

type aktRandom struct {
	state uint64
}

type Pair struct {
	RegistrationName string
	ActivationKey    string
	Format           string
}

func Generate(name string) (Pair, error) {
	if name == "" {
		var err error
		name, err = randomName(12)
		if err != nil {
			return Pair{}, err
		}
	}

	key, err := generateSignedKey(name, currentArmadilloDate(), randomSignatureSeed)
	if err != nil {
		return Pair{}, err
	}
	return Pair{RegistrationName: name, ActivationKey: key, Format: Format}, nil
}

func randomName(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(shortV3Alphabet))))
		if err != nil {
			return "", err
		}
		result[i] = shortV3Alphabet[n.Int64()]
	}
	return string(result), nil
}

func newAKTRandom(seed uint32) *aktRandom {
	return &aktRandom{state: uint64(seed)}
}

func (r *aktRandom) nextRange(n uint64) byte {
	r.state = (aktMult(r.state, 31415821) + 1) % 100000000
	return byte(((r.state / 10000) * n) / 10000)
}

func aktMult(p uint64, q uint64) uint64 {
	const m1 = 10000
	p1, p0 := p/m1, p%m1
	q1, q0 := q/m1, q%m1
	return (((p0*q1+p1*q0)%m1)*m1 + p0*q0) % 100000000
}

func cookText(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		out.WriteByte(c)
	}
	return out.String()
}

var aktCRC32Table [256]uint32

func init() {
	const poly32 uint32 = 0x04c11db7
	for x := range aktCRC32Table {
		r := reflectBits(uint32(x), 8) << 24
		for range 8 {
			if r&(1<<31) != 0 {
				r = (r << 1) ^ poly32
			} else {
				r <<= 1
			}
		}
		aktCRC32Table[x] = reflectBits(r, 32)
	}
}

func reflectBits(v uint32, bits int) uint32 {
	var out uint32
	for i := range bits {
		if v&(1<<i) != 0 {
			out |= 1 << (bits - 1 - i)
		}
	}
	return out
}

func aktCRC32(data []byte, crc uint32) uint32 {
	for _, b := range data {
		crc = aktCRC32Table[byte(crc)^b] ^ (crc >> 8)
	}
	return crc
}

func getKeyCRC(text string, period int) uint32 {
	cooked := cookText(text)
	if period == 1 {
		cooked = reverseString(cooked)
	} else if period > 1 {
		var grouped strings.Builder
		grouped.Grow(len(cooked))
		for i := 0; i < len(cooked); i += period {
			end := min(i+period, len(cooked))
			for j := end - 1; j >= i; j-- {
				grouped.WriteByte(cooked[j])
			}
		}
		cooked = grouped.String()
	}
	return aktCRC32([]byte(cooked), math.MaxUint32)
}

func reverseString(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

func currentArmadilloDate() uint16 {
	base, _ := time.Parse("2006-01-02", armadilloDateEpoch)
	return uint16(time.Since(base) / (24 * time.Hour))
}

func randomSignatureSeed() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint32(time.Now().Unix())
	}
	return binary.BigEndian.Uint32(b[:])
}

func generateSignedKey(name string, today uint16, seedSource func() uint32) (string, error) {
	if cookText(name) == "" {
		return "", errors.New("registration name cannot be empty")
	}

	privateKey, ok := new(big.Int).SetString(shortV3PrivateHex, 16)
	if !ok {
		return "", errors.New("invalid embedded private key")
	}
	pubBase, ok := new(big.Int).SetString(shortV3PrimeBase, 16)
	if !ok {
		return "", errors.New("invalid embedded public base")
	}

	keyBytes := make([]byte, 0, 24)
	keyBytes = binary.LittleEndian.AppendUint16(keyBytes, today)
	// The payload is little-endian, while the target parses the decrypted seed as
	// a big-endian dword before entering the core certificate gate.
	keyBytes = binary.LittleEndian.AppendUint32(keyBytes, bits.ReverseBytes32(shortV3CertSeed))

	rng := newAKTRandom(aktCRC32([]byte(cookText(name)), math.MaxUint32))
	for i := range keyBytes {
		keyBytes[i] ^= rng.nextRange(256)
	}

	size := shortV3LevelIndex + 4
	p := new(big.Int).Lsh(big.NewInt(1), uint(size*8))
	p.Add(p, big.NewInt(primeOffsets[shortV3LevelIndex]))
	pMinusOne := new(big.Int).Sub(new(big.Int).Set(p), big.NewInt(1))
	message := shortV3Message(keyBytes, name)

	a, b, err := makeElGamalSignature(message, privateKey, pubBase, p, pMinusOne, size, seedSource)
	if err != nil {
		return "", err
	}
	for range size {
		keyBytes = append(keyBytes, byte(new(big.Int).And(a, big.NewInt(0xff)).Uint64()))
		a.Rsh(a, 8)
		keyBytes = append(keyBytes, byte(new(big.Int).And(b, big.NewInt(0xff)).Uint64()))
		b.Rsh(b, 8)
	}

	return encodeShortV3Key(keyBytes), nil
}

func shortV3Message(keyBytes []byte, name string) *big.Int {
	buf := make([]byte, 0, len(keyBytes)+len(name))
	buf = append(buf, keyBytes...)
	buf = append(buf, []byte(cookText(name))...)
	sum := md5.Sum(buf)
	msg := new(big.Int)
	for i := 0; i < len(sum); i += 4 {
		msg.Lsh(msg, 32)
		msg.Add(msg, new(big.Int).SetUint64(uint64(binary.LittleEndian.Uint32(sum[i:]))))
	}
	return msg
}

func makeElGamalSignature(message, privateKey, pubBase, p, pMinusOne *big.Int, size int, seedSource func() uint32) (*big.Int, *big.Int, error) {
	minPart := big.NewInt(256)
	maxPart := new(big.Int).Lsh(big.NewInt(1), uint(size*8))
	seed := seedSource()
	if seed == 0 {
		seed = 1000
	}

	for attempts := range 1_000_000 {
		kSource := formatHex8(seed + uint32(attempts))
		k := new(big.Int)
		for x := range 5 {
			k.Lsh(k, 4)
			k.Add(k, new(big.Int).SetUint64(uint64(getKeyCRC(kSource, x+2))))
		}
		k.Mod(k, p)
		if k.Sign() <= 0 || new(big.Int).GCD(nil, nil, k, pMinusOne).Cmp(big.NewInt(1)) != 0 {
			continue
		}

		a := new(big.Int).Exp(pubBase, k, p)
		pa := new(big.Int).Mul(privateKey, a)
		diff := new(big.Int).Sub(message, pa)
		kInv := new(big.Int).ModInverse(k, pMinusOne)
		if kInv == nil {
			continue
		}
		b := new(big.Int).Mul(diff, kInv)
		b.Mod(b, pMinusOne)

		if a.Cmp(minPart) >= 0 && a.Cmp(maxPart) < 0 && b.Cmp(minPart) >= 0 && b.Cmp(maxPart) < 0 {
			return a, b, nil
		}
	}
	return nil, nil, errors.New("failed to produce bounded signature")
}

func formatHex8(v uint32) string {
	const hex = "0123456789ABCDEF"
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = hex[v&0xf]
		v >>= 4
	}
	return string(buf)
}

func encodeShortV3Key(keyBytes []byte) string {
	n := new(big.Int).SetBytes(keyBytes)
	zero := big.NewInt(0)
	thirtyTwo := big.NewInt(32)
	var out []byte
	dcount := 6
	for n.Cmp(zero) != 0 {
		mod := new(big.Int).Mod(n, thirtyTwo)
		n.Rsh(n, 5)
		nn := int(mod.Int64())
		if n.Cmp(zero) == 0 {
			if nn < 16 {
				out = append(out, shortV3Alphabet[nn+16])
				dcount--
			} else {
				out = append(out, shortV3Alphabet[nn])
				dcount--
				if dcount == 0 {
					out = append(out, '-')
					dcount = 6
				}
				out = append(out, shortV3Alphabet[16])
				dcount--
			}
		} else {
			out = append(out, shortV3Alphabet[nn])
			dcount--
			if dcount == 0 {
				out = append(out, '-')
				dcount = 6
			}
		}
	}
	for dcount > 0 {
		out = append(out, '0')
		dcount--
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}
