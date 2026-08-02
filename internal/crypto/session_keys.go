package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const minSessionRSABits = 2048

// ParseSessionRSAKeyPair parses PEM-encoded RSA private (PKCS#1 or PKCS#8)
// and public (PKIX) keys. Private and public must form a matching pair with
// at least minSessionRSABits.
func ParseSessionRSAKeyPair(privPEM, pubPEM string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	priv, err := parseRSAPrivateKeyPEM(privPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("SESSION_PRIVATE_KEY: %w", err)
	}
	pub, err := parseRSAPublicKeyPEM(pubPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("SESSION_PUBLIC_KEY: %w", err)
	}
	if err := validateRSAKeyPair(priv, pub); err != nil {
		return nil, nil, err
	}
	return priv, pub, nil
}

func normalizePEM(s string) string {
	s = strings.TrimSpace(s)
	// Env files often store PEMs as a single line with literal \n escapes.
	s = strings.ReplaceAll(s, `\n`, "\n")
	return s
}

func parseRSAPrivateKeyPEM(pemStr string) (*rsa.PrivateKey, error) {
	if commonstrings.IsEmpty(strings.TrimSpace(pemStr)) {
		return nil, errors.New("is required")
	}

	block, _ := pem.Decode(commonstrings.StringToBytes(normalizePEM(pemStr)))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("must be PKCS#1 or PKCS#8 RSA private key")
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("must be an RSA private key")
	}

	return key, nil
}

func parseRSAPublicKeyPEM(pemStr string) (*rsa.PublicKey, error) {
	if commonstrings.IsEmpty(strings.TrimSpace(pemStr)) {
		return nil, errors.New("is required")
	}

	block, _ := pem.Decode(commonstrings.StringToBytes(normalizePEM(pemStr)))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}

	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("must be a PKIX RSA public key")
	}

	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("must be an RSA public key")
	}

	return key, nil
}

func validateRSAKeyPair(priv *rsa.PrivateKey, pub *rsa.PublicKey) error {
	if priv.N.BitLen() < minSessionRSABits {
		return fmt.Errorf("SESSION_PRIVATE_KEY: RSA key must be at least %d bits", minSessionRSABits)
	}
	if pub.N.BitLen() < minSessionRSABits {
		return fmt.Errorf("SESSION_PUBLIC_KEY: RSA key must be at least %d bits", minSessionRSABits)
	}
	if priv.N.Cmp(pub.N) != 0 || priv.E != pub.E {
		return errors.New("SESSION_PRIVATE_KEY and SESSION_PUBLIC_KEY do not form a matching pair")
	}
	if priv.D == nil || priv.D.Cmp(big.NewInt(0)) <= 0 {
		return errors.New("SESSION_PRIVATE_KEY: invalid private exponent")
	}
	return nil
}
