package auth

import (
	"crypto/sha256"
	"encoding/hex"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// DeviceFingerprint returns a stable hex SHA-256 of User-Agent and client IP.
func DeviceFingerprint(userAgent, clientIP string) string {
	sum := sha256.Sum256(commonstrings.StringToBytes(userAgent + "|" + clientIP))
	return hex.EncodeToString(sum[:])
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256(commonstrings.StringToBytes(raw))
	return hex.EncodeToString(sum[:])
}
