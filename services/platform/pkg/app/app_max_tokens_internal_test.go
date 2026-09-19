package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToolcallMaxTokensOptionsIsNilWhenUnset(t *testing.T) {
	opts := toolcallMaxTokensOptions(&Config{})

	assert.Nil(t, opts, "0 (the zero value) means \"use toolcall.New's own default\" - no option at all")
}

func TestToolcallMaxTokensOptionsBuildsOneOptionWhenPositive(t *testing.T) {
	opts := toolcallMaxTokensOptions(&Config{LLM: LLM{MaxTokens: 4000}})

	assert.Len(t, opts, 1)
}

func TestJsonmodeMaxTokensOptionsIsNilWhenUnset(t *testing.T) {
	opts := jsonmodeMaxTokensOptions(&Config{})

	assert.Nil(t, opts, "0 (the zero value) means \"use jsonmode.New's own default\" - no option at all")
}

func TestJsonmodeMaxTokensOptionsBuildsOneOptionWhenPositive(t *testing.T) {
	opts := jsonmodeMaxTokensOptions(&Config{LLM: LLM{MaxTokens: 4000}})

	assert.Len(t, opts, 1)
}

func TestPickOptionsIsNilWhenUnset(t *testing.T) {
	opts := pickOptions(&Config{})

	assert.Nil(t, opts, "0 (the zero value) means \"use pick.New's own default\" - no option at all")
}

func TestPickOptionsBuildsOneOptionWhenPositive(t *testing.T) {
	opts := pickOptions(&Config{LLM: LLM{PickMaxTokens: 500}})

	assert.Len(t, opts, 1)
}
