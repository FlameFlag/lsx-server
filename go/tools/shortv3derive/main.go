package main

import (
	"errors"
	"flag"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
)

const (
	defaultPublic = "9CC50E4D25416464B9"
	defaultBase   = "F3C7E00A4B58155299"
	defaultKnownX = "70301169DE7C75D66F"
	defaultLevel  = 25
)

var primeOffsets = [...]int64{15, 15, 21, 81, 3103, 3643, 2191, 9691, 2887}

func main() {
	publicHex := flag.String("public", defaultPublic, "ShortV3 public certificate y in hex")
	baseHex := flag.String("base", defaultBase, "ShortV3 generator/base g in hex")
	knownHex := flag.String("known-private", defaultKnownX, "known recovered exponent x to verify and document")
	level := flag.Int("level", defaultLevel, "ShortV3 level; Lemonade2 uses 25")
	solve := flag.Bool("solve", false, "attempt built-in Pohlig-Hellman/BSGS solve; only practical for small subgroup factors")
	flag.Parse()

	params, err := newShortV3Params(*publicHex, *baseHex, *level)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	factors := make(map[string]int)
	factor(params.order, factors)

	factorText := make([]string, 0, len(factors))
	for _, factor := range sortedFactors(factors) {
		exponent := factors[factor.String()]
		if exponent == 1 {
			factorText = append(factorText, factor.Text(10))
		} else {
			factorText = append(factorText, fmt.Sprintf("%s^%d", factor.Text(10), exponent))
		}
	}

	fmt.Printf("level: %d\n", *level)
	fmt.Printf("modulus p: 0x%X\n", params.modulus)
	fmt.Printf("order p-1: 0x%X\n", params.order)
	fmt.Printf("order factors: %v\n", factorText)

	known, ok := new(big.Int).SetString(*knownHex, 16)
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid known private exponent hex %q\n", *knownHex)
		os.Exit(1)
	}
	verified := new(big.Int).Exp(params.base, known, params.modulus).Cmp(params.public) == 0
	fmt.Printf("known private exponent x: 0x%X\n", known)
	fmt.Printf("known exponent verifies public certificate: %v\n", verified)
	fmt.Println("sage reproduction:")
	fmt.Printf("  p = 0x%X; g = 0x%X; y = 0x%X; discrete_log(Mod(y, p), Mod(g, p))\n",
		params.modulus, params.base, params.public)
	fmt.Println("cado-nfs reproduction input:")
	fmt.Printf("  p=0x%X g=0x%X y=0x%X order_factor=0x%X\n",
		params.modulus, params.base, params.public, params.order)

	if !*solve {
		return
	}
	private, err := solvePrivate(params, factors)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("solved private exponent x: 0x%X\n", private)
}

type shortV3Params struct {
	public  *big.Int
	base    *big.Int
	modulus *big.Int
	order   *big.Int
}

func newShortV3Params(publicHex string, baseHex string, level int) (shortV3Params, error) {
	idx := level - 20
	if idx < 0 || idx >= len(primeOffsets) {
		return shortV3Params{}, fmt.Errorf("unsupported ShortV3 level %d", level)
	}
	y, ok := new(big.Int).SetString(publicHex, 16)
	if !ok {
		return shortV3Params{}, fmt.Errorf("invalid public certificate hex %q", publicHex)
	}
	g, ok := new(big.Int).SetString(baseHex, 16)
	if !ok {
		return shortV3Params{}, fmt.Errorf("invalid base hex %q", baseHex)
	}
	size := idx + 4
	p := new(big.Int).Lsh(big.NewInt(1), uint(size*8))
	p.Add(p, big.NewInt(primeOffsets[idx]))
	n := new(big.Int).Sub(new(big.Int).Set(p), big.NewInt(1))
	return shortV3Params{public: y, base: g, modulus: p, order: n}, nil
}

func solvePrivate(params shortV3Params, factors map[string]int) (*big.Int, error) {
	x, err := pohligHellman(params.base, params.public, params.modulus, params.order, factors)
	if err != nil {
		return nil, err
	}
	if new(big.Int).Exp(params.base, x, params.modulus).Cmp(params.public) != 0 {
		return nil, errors.New("derived exponent does not reproduce public certificate")
	}
	return x, nil
}

func pohligHellman(g, h, p, order *big.Int, factors map[string]int) (*big.Int, error) {
	residues := make([]*big.Int, 0, len(factors))
	moduli := make([]*big.Int, 0, len(factors))
	for _, q := range sortedFactors(factors) {
		e := factors[q.String()]
		qe := new(big.Int).Exp(q, big.NewInt(int64(e)), nil)
		x, err := primePowerLog(g, h, p, order, q, e)
		if err != nil {
			return nil, err
		}
		residues = append(residues, x)
		moduli = append(moduli, qe)
	}
	return crt(residues, moduli)
}

func primePowerLog(g, h, p, order, q *big.Int, e int) (*big.Int, error) {
	x := big.NewInt(0)
	qPow := big.NewInt(1)
	gamma := new(big.Int).Exp(g, new(big.Int).Div(order, q), p)
	for j := 0; j < e; j++ {
		denom := new(big.Int).Mul(qPow, q)
		exp := new(big.Int).Div(order, denom)
		gx := new(big.Int).Exp(g, x, p)
		gxInv := new(big.Int).ModInverse(gx, p)
		if gxInv == nil {
			return nil, errors.New("generator inverse does not exist")
		}
		t := new(big.Int).Mul(h, gxInv)
		t.Mod(t, p)
		t.Exp(t, exp, p)

		digit, err := discreteLogBSGS(gamma, t, p, q)
		if err != nil {
			return nil, fmt.Errorf("prime-power digit %d for q=%s: %w", j, q.Text(10), err)
		}
		x.Add(x, new(big.Int).Mul(digit, qPow))
		qPow.Mul(qPow, q)
	}
	return x, nil
}

func discreteLogBSGS(g, h, p, order *big.Int) (*big.Int, error) {
	if order.BitLen() > 31 {
		return nil, fmt.Errorf("subgroup order %s is too large for this BSGS implementation", order.Text(10))
	}
	m := int64(math.Ceil(math.Sqrt(float64(order.Int64()))))
	baby := make(map[string]int64, m)
	e := big.NewInt(1)
	for j := int64(0); j < m; j++ {
		baby[e.String()] = j
		e.Mul(e, g).Mod(e, p)
	}

	gM := new(big.Int).Exp(g, big.NewInt(m), p)
	factor := new(big.Int).ModInverse(gM, p)
	if factor == nil {
		return nil, errors.New("giant-step factor inverse does not exist")
	}
	gamma := new(big.Int).Set(h)
	for i := int64(0); i <= m; i++ {
		if j, ok := baby[gamma.String()]; ok {
			x := big.NewInt(0).Add(big.NewInt(0).Mul(big.NewInt(i), big.NewInt(m)), big.NewInt(j))
			if x.Cmp(order) < 0 {
				return x, nil
			}
		}
		gamma.Mul(gamma, factor).Mod(gamma, p)
	}
	return nil, errors.New("discrete log not found")
}

func crt(residues []*big.Int, moduli []*big.Int) (*big.Int, error) {
	x := big.NewInt(0)
	m := big.NewInt(1)
	for i := range residues {
		mi := moduli[i]
		delta := new(big.Int).Sub(residues[i], x)
		delta.Mod(delta, mi)
		inv := new(big.Int).ModInverse(new(big.Int).Mod(m, mi), mi)
		if inv == nil {
			return nil, errors.New("CRT moduli are not coprime")
		}
		t := delta.Mul(delta, inv)
		t.Mod(t, mi)
		x.Add(x, new(big.Int).Mul(m, t))
		m.Mul(m, mi)
		x.Mod(x, m)
	}
	return x, nil
}

func factor(n *big.Int, out map[string]int) {
	if n.Cmp(big.NewInt(1)) == 0 {
		return
	}
	if n.ProbablyPrime(20) {
		out[n.String()]++
		return
	}
	for _, small := range smallPrimes() {
		p := big.NewInt(int64(small))
		if new(big.Int).Mod(n, p).Sign() == 0 {
			out[p.String()]++
			factor(new(big.Int).Div(n, p), out)
			return
		}
	}
	d := pollardRho(n)
	factor(d, out)
	factor(new(big.Int).Div(n, d), out)
}

func pollardRho(n *big.Int) *big.Int {
	if new(big.Int).And(n, big.NewInt(1)).Sign() == 0 {
		return big.NewInt(2)
	}
	one := big.NewInt(1)
	two := big.NewInt(2)
	for c := int64(1); ; c++ {
		x := big.NewInt(2)
		y := big.NewInt(2)
		cc := big.NewInt(c)
		d := big.NewInt(1)
		for d.Cmp(one) == 0 {
			x = rhoStep(x, cc, n)
			y = rhoStep(rhoStep(y, cc, n), cc, n)
			d.Sub(x, y).Abs(d).GCD(nil, nil, d, n)
		}
		if d.Cmp(n) != 0 && d.Cmp(two) >= 0 {
			return d
		}
	}
}

func rhoStep(x, c, n *big.Int) *big.Int {
	out := new(big.Int).Mul(x, x)
	out.Add(out, c)
	out.Mod(out, n)
	return out
}

func smallPrimes() []int {
	const limit = 10000
	composite := make([]bool, limit+1)
	var primes []int
	for i := 2; i <= limit; i++ {
		if composite[i] {
			continue
		}
		primes = append(primes, i)
		for j := i * i; j <= limit; j += i {
			composite[j] = true
		}
	}
	return primes
}

func sortedFactors(factors map[string]int) []*big.Int {
	result := make([]*big.Int, 0, len(factors))
	for text := range factors {
		factor, ok := new(big.Int).SetString(text, 10)
		if !ok {
			panic("bad factor")
		}
		result = append(result, factor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Cmp(result[j]) < 0 })
	return result
}
