package utils

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashMPIN returns a bcrypt hash of the supplied MPIN.
func HashMPIN(mpin string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(mpin), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// IsHashedMPIN reports whether stored already holds a bcrypt hash rather than a
// legacy plaintext MPIN.
func IsHashedMPIN(stored string) bool {
	return strings.HasPrefix(stored, "$2a$") ||
		strings.HasPrefix(stored, "$2b$") ||
		strings.HasPrefix(stored, "$2y$")
}

// CheckMPIN compares a candidate MPIN against the stored value. Accounts created
// before MPINs were hashed still hold plaintext, so those fall back to a direct
// comparison; the caller is expected to re-hash on a successful legacy match.
func CheckMPIN(stored, candidate string) bool {
	if stored == "" {
		return false
	}
	if IsHashedMPIN(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(candidate)) == nil
	}
	return stored == candidate
}
