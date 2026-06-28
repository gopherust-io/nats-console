package crypto_test

import (
	"testing"

	"github.com/gopherust-io/nats-consol/internal/crypto"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	enc, err := crypto.New("test-encryption-key-32chars!")
	if err != nil {
		t.Fatal(err)
	}

	plain := "super-secret-nats-token"
	cipher, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !crypto.IsEncrypted(cipher) {
		t.Fatal("expected encrypted prefix")
	}

	out, err := enc.Decrypt(cipher)
	if err != nil {
		t.Fatal(err)
	}
	if out != plain {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestPlaintextPassthrough(t *testing.T) {
	enc, err := crypto.New("another-valid-key-here!!")
	if err != nil {
		t.Fatal(err)
	}
	out, err := enc.Decrypt("plain-token")
	if err != nil {
		t.Fatal(err)
	}
	if out != "plain-token" {
		t.Fatalf("got %q", out)
	}
}
