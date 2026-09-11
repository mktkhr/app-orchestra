package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPasswordRoundTripsThroughVerifyPassword(t *testing.T) {
	encoded := hashPassword("correct horse battery staple")

	ok, err := verifyPassword("correct horse battery staple", encoded)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = verifyPassword("wrong password", encoded)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestHashPasswordProducesADifferentSaltEachTime(t *testing.T) {
	first := hashPassword("correct horse battery staple")
	second := hashPassword("correct horse battery staple")

	assert.NotEqual(t, first, second)
}

func TestVerifyPasswordRejectsAMalformedHash(t *testing.T) {
	_, err := verifyPassword("whatever", "not-a-hash")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}

func TestVerifyPasswordRejectsAnUnknownAlgorithm(t *testing.T) {
	_, err := verifyPassword("whatever", "$bcrypt$v=1$m=1,t=1,p=1$c2FsdA$aGFzaA")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}

func TestVerifyPasswordRejectsUnparsableParameters(t *testing.T) {
	_, err := verifyPassword("whatever", "$argon2id$v=19$not-parameters$c2FsdA$aGFzaA")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}

func TestVerifyPasswordRejectsAMalformedSalt(t *testing.T) {
	_, err := verifyPassword("whatever", "$argon2id$v=19$m=65536,t=1,p=4$not-base64!!$aGFzaA")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}

func TestVerifyPasswordRejectsAHashOfTheWrongLength(t *testing.T) {
	// A validly-encoded, but too-short, stored hash - "c2FsdA" decodes to
	// 4 bytes, not argonKeyLen.
	_, err := verifyPassword("whatever", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$c2FsdA")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}

func TestVerifyPasswordRejectsAMalformedStoredHash(t *testing.T) {
	_, err := verifyPassword("whatever", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$not-base64!!")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedHash)
}
