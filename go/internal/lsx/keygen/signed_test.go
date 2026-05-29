package keygen

import (
	"math/big"
	"testing"
)

func TestShortV3PrivateKeyMatchesRecoveredPublicCertificate(t *testing.T) {
	privateKey, ok := new(big.Int).SetString(shortV3PrivateHex, 16)
	if !ok {
		t.Fatal("invalid private key hex")
	}
	base, ok := new(big.Int).SetString(shortV3PrimeBase, 16)
	if !ok {
		t.Fatal("invalid base hex")
	}
	want, ok := new(big.Int).SetString(shortV3PublicCertHex, 16)
	if !ok {
		t.Fatal("invalid public certificate hex")
	}

	size := shortV3LevelIndex + 4
	modulus := new(big.Int).Lsh(big.NewInt(1), uint(size*8))
	modulus.Add(modulus, big.NewInt(primeOffsets[shortV3LevelIndex]))

	got := new(big.Int).Exp(base, privateKey, modulus)
	if got.Cmp(want) != 0 {
		t.Fatalf("public cert = %X, want %X", got, want)
	}
}

func TestGenerateSignedKeyKnownVector(t *testing.T) {
	key, err := generateSignedKey("TestName", 10011, func() uint32 { return 1000 })
	if err != nil {
		t.Fatal(err)
	}
	const want = "0000PP-FZYKGQ-JABWAK-Q6XMT6-U0U72Q-CD4Y50-JTAV0G"
	if key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
}

func TestGenerateSignedKeyRejectsEmptyCookedName(t *testing.T) {
	_, err := generateSignedKey(" \t\r\n", 10011, func() uint32 { return 1000 })
	if err == nil {
		t.Fatal("expected error for empty cooked name")
	}
}

func TestGenerateUsesProvidedName(t *testing.T) {
	pair, err := Generate("TestName")
	if err != nil {
		t.Fatal(err)
	}
	if pair.RegistrationName != "TestName" {
		t.Fatalf("registration name = %q", pair.RegistrationName)
	}
	if pair.ActivationKey == "" {
		t.Fatal("empty key")
	}
	if pair.Format != Format {
		t.Fatalf("format = %q", pair.Format)
	}
}

func TestGenerateRandomName(t *testing.T) {
	pair, err := Generate("")
	if err != nil {
		t.Fatal(err)
	}
	if pair.RegistrationName == "" || pair.ActivationKey == "" {
		t.Fatalf("empty generated pair: %#v", pair)
	}
}
