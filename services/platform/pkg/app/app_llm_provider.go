// app_llm_provider.go: the second chat backend's own composition
// (docs: Anthropic Messages API planner) - ProviderLlamaSwap/
// ProviderAnthropic, ErrInvalidLLMProvider/ErrMissingAnthropicAPIKey,
// llmConfigured and newChatCompleter - split out of app.go
// (harness/quality/filelen.sh's 1000-line guard: app.go was already near
// its own cap before this subproject existed), the same split
// app_staging.go's own doc comment describes. The package's own doc
// comment is app.go's.

package app

import (
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat/anthropic"
)

// ProviderLlamaSwap and ProviderAnthropic are LLM.Provider's two non-empty
// values, mirroring internal/infra/config.LLMProviderLlamaSwap and
// LLMProviderAnthropic.
const (
	ProviderLlamaSwap = "llamaswap"
	ProviderAnthropic = "anthropic"
)

// ErrInvalidLLMProvider is returned by New when Config.LLM.Provider is set
// to anything other than "" (ProviderLlamaSwap), ProviderLlamaSwap or
// ProviderAnthropic - mirroring ErrInvalidLLMMode's own refusal to fall
// back to the default silently. cmd/api never reaches this: it always goes
// through config.Load's own validation first (config.ErrInvalidLLMProvider)
// - the same defence-in-depth ErrInvalidLLMMode already provides.
var ErrInvalidLLMProvider = errors.New("invalid LLM.Provider, want \"\", \"llamaswap\" or \"anthropic\"")

// ErrMissingAnthropicAPIKey is returned by New when Config.LLM.Provider is
// ProviderAnthropic but Config.LLM.AnthropicAPIKey is empty - mirroring
// config.ErrMissingAnthropicAPIKey the same way ErrMissingJevAPIKey mirrors
// config.ErrMissingJevAPIKey.
var ErrMissingAnthropicAPIKey = errors.New("LLM.AnthropicAPIKey is required when LLM.Provider is \"anthropic\"")

// llmConfigured is whether newPlanner (and stagingOptions) should build a
// real planner against cfg.LLM rather than falling back to the stub.
//
// "" and ProviderLlamaSwap are treated identically - both mean "the
// default backend" - because internal/infra/config.Load never actually
// leaves LLMProvider empty: ORCHESTRA_LLM_PROVIDER unset still parses to
// LLMProviderLlamaSwap (config.parseLLMProvider), so cmd/api always passes
// a non-empty Provider through even when no LLM is configured at all. Were
// this function to treat "Provider set to anything" as configured, every
// real deployment with ORCHESTRA_LLM_BASE_URL unset would stop falling
// back to the stub and instead build a chat.Client with an empty base URL
// - exactly the acceptance-e2e regression this comment now documents.
//
// Any other Provider value - ProviderAnthropic (needs no base URL of its
// own, AnthropicBaseURL's own doc comment) or an unknown one - counts as
// configured, so a typo'd Provider still reaches newChatCompleter's own
// ErrInvalidLLMProvider instead of silently falling back to the stub.
func llmConfigured(cfg *Config) bool {
	return cfg.LLM.BaseURL != "" || (cfg.LLM.Provider != "" && cfg.LLM.Provider != ProviderLlamaSwap)
}

// newChatCompleter builds the chat.Completer newPlanner and newPicker (via
// newChatCompleter's own two callers, PickerLocal in newPicker and
// newHybridPicker) both run their planning calls through: a
// chat.Client over cfg.LLM.BaseURL/APIKey/Model for "" or
// ProviderLlamaSwap (today's OpenAI-compatible transport, unchanged), or
// an anthropic.Client over cfg.LLM.AnthropicAPIKey/Model/AnthropicThinking
// for ProviderAnthropic. Both toolcall.Planner and pick.Picker accept a
// chat.Completer rather than either concrete type, so this is the one
// place the choice is made.
func newChatCompleter(cfg *Config) (chat.Completer, error) {
	switch cfg.LLM.Provider {
	case "", ProviderLlamaSwap:
		return chat.New(chat.Config{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model}), nil
	case ProviderAnthropic:
		if cfg.LLM.AnthropicAPIKey == "" {
			return nil, ErrMissingAnthropicAPIKey
		}

		return anthropic.New(anthropic.Config{
			BaseURL: cfg.LLM.AnthropicBaseURL, APIKey: cfg.LLM.AnthropicAPIKey, Model: cfg.LLM.Model,
			ThinkingEnabled: cfg.LLM.AnthropicThinking,
		}), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidLLMProvider, cfg.LLM.Provider)
	}
}
