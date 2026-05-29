package main

import (
	"math/big"
	"testing"
)

func TestKnownShortV3PrivateExponentVerifies(t *testing.T) {
	params, err := newShortV3Params(defaultPublic, defaultBase, defaultLevel)
	if err != nil {
		t.Fatal(err)
	}
	known, ok := new(big.Int).SetString(defaultKnownX, 16)
	if !ok {
		t.Fatal("bad known exponent")
	}
	got := new(big.Int).Exp(params.base, known, params.modulus)
	if got.Cmp(params.public) != 0 {
		t.Fatalf("public cert = %X, want %X", got, params.public)
	}
}

func TestShortV3OrderFactorization(t *testing.T) {
	params, err := newShortV3Params(defaultPublic, defaultBase, defaultLevel)
	if err != nil {
		t.Fatal(err)
	}
	factors := make(map[string]int)
	factor(params.order, factors)
	if factors["2"] != 1 || factors["2361183241434822608669"] != 1 || len(factors) != 2 {
		t.Fatalf("factors = %#v", factors)
	}
}
