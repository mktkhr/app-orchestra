package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/infra/config"
)

// TestLoadLLMProviderDefaultsToLlamaSwap is AC-style: a run that never
// mentions ORCHESTRA_LLM_PROVIDER at all - every deployment before this
// subproject existed - must load exactly LLMProviderLlamaSwap, so
// pkg/app.newChatCompleter takes the same branch it always has.
func TestLoadLLMProviderDefaultsToLlamaSwap(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.LLMProviderLlamaSwap, cfg.LLMProvider)
	assert.Empty(t, cfg.AnthropicAPIKey)
}

func TestLoadReadsLLMProviderAnthropicWithAPIKey(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_PROVIDER", "anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.LLMProviderAnthropic, cfg.LLMProvider)
	assert.Equal(t, "sk-ant-test", cfg.AnthropicAPIKey)
}

func TestLoadRejectsAnUnknownLLMProvider(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_PROVIDER", "bedrock")

	_, err := config.Load()

	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrInvalidLLMProvider)
}

func TestLoadRejectsLLMProviderAnthropicWithoutAnAPIKey(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ORCHESTRA_LLM_PROVIDER", "anthropic")

	_, err := config.Load()

	require.Error(t, err)
	require.ErrorIs(t, err, config.ErrMissingAnthropicAPIKey)
}

// TestLoadIgnoresAnthropicAPIKeyForLlamaSwap documents that
// ANTHROPIC_API_KEY left set after switching back to the default provider
// is read but never validated - the same "inert, not an error" rule
// config.Config.JevAPIKey's own doc comment already gives for a value left
// set when it no longer applies.
func TestLoadIgnoresAnthropicAPIKeyForLlamaSwap(t *testing.T) {
	t.Setenv("ORCHESTRA_DB_PATH", "/tmp/orchestra-test.db")
	t.Setenv("ORCHESTRA_ADMIN_PASSWORD", "correct horse battery staple")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, config.LLMProviderLlamaSwap, cfg.LLMProvider)
	assert.Equal(t, "sk-ant-test", cfg.AnthropicAPIKey)
}
