package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFillOptionsBothArmsOffByDefault(t *testing.T) {
	opts, err := newFillOptions(&Config{})

	require.NoError(t, err)
	assert.Empty(t, opts, "the zero value Fill (both arms off) builds no options at all")
}

func TestNewFillOptionsSkipEmptyAlone(t *testing.T) {
	opts, err := newFillOptions(&Config{Fill: Fill{SkipEmpty: true}})

	require.NoError(t, err)
	assert.Len(t, opts, 1)
}

func TestNewFillOptionsEnumJevWithoutAKeyIsAnError(t *testing.T) {
	_, err := newFillOptions(&Config{Fill: Fill{Enum: FillEnumJev}})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingJevAPIKey)
}

func TestNewFillOptionsEnumJevWithAKeyBuildsOneOption(t *testing.T) {
	opts, err := newFillOptions(&Config{Fill: Fill{Enum: FillEnumJev, JevAPIKey: "test-key"}})

	require.NoError(t, err)
	assert.Len(t, opts, 1)
}

func TestNewFillOptionsBothArmsTogetherBuildTwoOptions(t *testing.T) {
	opts, err := newFillOptions(&Config{Fill: Fill{SkipEmpty: true, Enum: FillEnumJev, JevAPIKey: "test-key"}})

	require.NoError(t, err)
	assert.Len(t, opts, 2, "the two arms are independent - both flags set builds both options")
}

func TestNewFillOptionsRejectsAnUnknownEnumValue(t *testing.T) {
	_, err := newFillOptions(&Config{Fill: Fill{Enum: "remote"}})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidFillEnum)
}

func TestNewFillOptionsEnumRefusalBuildsOneOption(t *testing.T) {
	opts, err := newFillOptions(&Config{Fill: Fill{Enum: FillEnumJev, JevAPIKey: "test-key", EnumRefusal: true}})

	require.NoError(t, err)
	assert.Len(t, opts, 1, "EnumRefusal configures the same one jev.Filler option, not an extra one")
}

func TestNewFillOptionsEnumUnsetWordingWideBuildsOneOption(t *testing.T) {
	opts, err := newFillOptions(&Config{
		Fill: Fill{Enum: FillEnumJev, JevAPIKey: "test-key", EnumUnsetWording: FillEnumUnsetWordingWide},
	})

	require.NoError(t, err)
	assert.Len(t, opts, 1, "EnumUnsetWording configures the same one jev.Filler option, not an extra one")
}
