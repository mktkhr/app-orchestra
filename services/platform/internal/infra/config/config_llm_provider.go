// config_llm_provider.go: the second chat backend's own configuration
// (ORCHESTRA_LLM_PROVIDER, ANTHROPIC_API_KEY, ORCHESTRA_ANTHROPIC_THINKING)
// - split out of config.go
// (harness/quality/filelen.sh's 1000-line guard: config.go was already
// near its own cap before this subproject existed), the same reason
// config_gate.go, config_hybrid.go and config_service_router.go each
// exist. The package's own doc comment is config.go's.

package config

import (
	"errors"
	"fmt"
	"os"
)

// LLMProviderLlamaSwap and LLMProviderAnthropic are ORCHESTRA_LLM_PROVIDER's
// two accepted values: pkg/app.newChatCompleter's choice between
// internal/adapter/planner/chat.Client (an OpenAI-compatible endpoint such
// as llama-swap - LLMProviderLlamaSwap, the default) and
// internal/adapter/planner/chat/anthropic.Client (the Anthropic Messages
// API - LLMProviderAnthropic). Both planner stages (toolcall.Planner,
// pick.Picker) run unchanged behind either, since each takes a
// chat.Completer rather than a concrete transport.
const (
	LLMProviderLlamaSwap = "llamaswap"
	LLMProviderAnthropic = "anthropic"
)

// ErrInvalidLLMProvider is wrapped into the error returned when
// ORCHESTRA_LLM_PROVIDER names anything other than LLMProviderLlamaSwap or
// LLMProviderAnthropic - the same reasoning ErrInvalidLLMMode already
// applies to ORCHESTRA_LLM_MODE: a typo'd provider fails startup rather
// than silently falling back to the default.
var ErrInvalidLLMProvider = errors.New(
	"invalid ORCHESTRA_LLM_PROVIDER, want " + LLMProviderLlamaSwap + " or " + LLMProviderAnthropic,
)

// ErrMissingAnthropicAPIKey is returned when ORCHESTRA_LLM_PROVIDER is
// LLMProviderAnthropic but ANTHROPIC_API_KEY is unset or empty -
// internal/adapter/planner/chat/anthropic has no route to the Messages API
// without one, the same reasoning ErrMissingJevAPIKey already applies to
// ORCHESTRA_JEV_API_KEY.
var ErrMissingAnthropicAPIKey = errors.New("ANTHROPIC_API_KEY is required when ORCHESTRA_LLM_PROVIDER=anthropic")

// anthropicThinkingOff and anthropicThinkingOn are
// ORCHESTRA_ANTHROPIC_THINKING's two accepted values - unexported the same
// way plannerThinkingOff/plannerThinkingOn (config.go) are, since nothing
// outside parseAnthropicThinking needs the raw string once
// Config.AnthropicThinking (a bool) exists.
const (
	anthropicThinkingOff = "off"
	anthropicThinkingOn  = "on"
)

// ErrInvalidAnthropicThinking is wrapped into the error returned when
// ORCHESTRA_ANTHROPIC_THINKING names anything other than "off" or "on" -
// the same reasoning ErrInvalidLLMProvider already applies to
// ORCHESTRA_LLM_PROVIDER: a typo'd value fails startup rather than
// silently falling back to the default.
var ErrInvalidAnthropicThinking = errors.New(
	"invalid ORCHESTRA_ANTHROPIC_THINKING, want " + anthropicThinkingOff + " or " + anthropicThinkingOn,
)

// parseAnthropicThinking reads ORCHESTRA_ANTHROPIC_THINKING: false
// (thinking left disabled for a model whose behaviorFor entry disables it
// by default - internal/adapter/planner/chat/anthropic/params.go - today's
// behaviour, byte-identical) when unset or "off", true when "on" -
// anything else fails startup rather than silently falling back to the
// default, the same reasoning parsePlannerThinking (config.go) already
// applies to ORCHESTRA_PLANNER_THINKING.
func parseAnthropicThinking(raw string) (bool, error) {
	switch raw {
	case "", anthropicThinkingOff:
		return false, nil
	case anthropicThinkingOn:
		return true, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrInvalidAnthropicThinking, raw)
	}
}

// parseLLMProvider reads ORCHESTRA_LLM_PROVIDER: LLMProviderLlamaSwap when
// unset, or exactly LLMProviderLlamaSwap or LLMProviderAnthropic otherwise
// - anything else fails startup rather than silently falling back to the
// default, the same reasoning parseLLMMode already applies to
// ORCHESTRA_LLM_MODE.
func parseLLMProvider(raw string) (string, error) {
	switch raw {
	case "":
		return LLMProviderLlamaSwap, nil
	case LLMProviderLlamaSwap, LLMProviderAnthropic:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidLLMProvider, raw)
	}
}

// loadLLMProvider reads ORCHESTRA_LLM_PROVIDER, ANTHROPIC_API_KEY and
// ORCHESTRA_ANTHROPIC_THINKING and applies them to cfg, isolating loadLLM
// itself from both the os.Getenv calls and their own validation - the same
// reason loadLLM's own doc comment gives for existing at all.
// ANTHROPIC_API_KEY is required exactly when the provider is
// LLMProviderAnthropic (ErrMissingAnthropicAPIKey); a value left set after
// switching back to LLMProviderLlamaSwap is read but never validated, the
// same "inert, not an error" rule loadPicker's own doc comment already
// gives ORCHESTRA_JEV_API_KEY - ORCHESTRA_ANTHROPIC_THINKING is read and
// validated the same inert way, since it only ever changes anything for
// internal/adapter/planner/chat/anthropic.Client, which pkg/app.
// newChatCompleter only builds when the provider is LLMProviderAnthropic.
func loadLLMProvider(cfg *Config) error {
	provider, err := parseLLMProvider(os.Getenv("ORCHESTRA_LLM_PROVIDER"))
	if err != nil {
		return err
	}

	cfg.LLMProvider = provider

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if provider == LLMProviderAnthropic && anthropicAPIKey == "" {
		return ErrMissingAnthropicAPIKey
	}

	cfg.AnthropicAPIKey = anthropicAPIKey

	thinking, err := parseAnthropicThinking(os.Getenv("ORCHESTRA_ANTHROPIC_THINKING"))
	if err != nil {
		return err
	}

	cfg.AnthropicThinking = thinking

	return nil
}
