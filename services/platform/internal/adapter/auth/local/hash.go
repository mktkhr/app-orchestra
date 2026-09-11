package local

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// The argon2id parameters this package hashes with. Chosen from the
// OWASP password-storage cheat sheet's argon2id baseline (m=19MiB..64MiB,
// t=1..2, p=1..4; RFC 9106 §4 recommends the same range for the
// single-server, no-secondary-defence case this platform is in) rather
// than argon2's own package-level defaults, which target a different
// profile (interactive key derivation, not a password store): 64 MiB and
// a single pass keep a sign-in under a second on ordinary hardware while
// staying far more expensive to brute-force than any KDF with a smaller
// memory cost, and threads=4 matches a typical small server's core count
// without needing a config knob nothing else in this codebase has either.
const (
	argonMemoryKiB = 64 * 1024 // 64 MiB
	argonTime      = 1
	argonThreads   = 4
	argonKeyLen    = 32
)

// ErrMalformedHash is returned when a stored password hash is not in the
// shape hashPassword produces - something that should never happen with a
// hash this package wrote itself, but a stored string is a stored string.
var ErrMalformedHash = errors.New("malformed password hash")

// hashPassword returns password's argon2id hash, encoded as a single
// string carrying its own salt and parameters (the PHC string format
// argon2's own reference implementation uses) so verifyPassword never has
// to be told what hashPassword used - the two travel together, and a
// future change to argonMemoryKiB does not break a hash stored under the
// old one. No error return: rand.Text() (Go 1.24+, see
// internal/adapter/repository/sqlite/id.go for the same reasoning) and
// argon2.IDKey cannot fail.
func hashPassword(password string) string {
	salt := []byte(rand.Text())

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
}

// verifyPassword reports whether password matches encoded, a string
// hashPassword produced. A malformed encoded value - never one this
// package wrote - is reported as ErrMalformedHash rather than treated as a
// non-match, so a corrupted row fails loudly instead of locking an account
// out silently.
func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, fmt.Errorf("%w: %q", ErrMalformedHash, encoded)
	}

	var memory, iterations uint32

	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false, fmt.Errorf("%w: %w", ErrMalformedHash, err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrMalformedHash, err)
	}

	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrMalformedHash, err)
	}

	// argonKeyLen, not uint32(len(want)): the output length is this
	// package's own choice, not the stored hash's to dictate - a want of
	// any other length is a corrupted row, caught by the length check
	// below rather than fed back into IDKey (which would also need an
	// int-to-uint32 conversion gosec flags as a possible overflow, for a
	// length that should never come from untrusted input anyway).
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, argonKeyLen)
	if len(want) != len(got) {
		return false, fmt.Errorf("%w: unexpected hash length", ErrMalformedHash)
	}

	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
