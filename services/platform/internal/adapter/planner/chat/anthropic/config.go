// Package anthropic implements chat.Completer over the Anthropic Messages
// API (https://api.anthropic.com/v1/messages) - the second chat backend
// the planner can run against, alongside
// internal/adapter/planner/chat.Client's OpenAI-compatible one
// (ORCHESTRA_LLM_PROVIDER=anthropic, internal/infra/config.Config.LLMProvider).
// It knows nothing about usecase.Tool or usecase.Decision, the same
// separation internal/adapter/planner/chat's own package doc comment
// describes for its sibling: only chat.Request in, chat.Response out, so
// internal/adapter/planner/toolcall and internal/adapter/planner/pick work
// unchanged against either backend behind chat.Completer.
package anthropic

// defaultBaseURL is the Anthropic Messages API's own base, used when
// Config.BaseURL is left empty - every real deployment; only a test
// fixture server ever sets BaseURL.
const defaultBaseURL = "https://api.anthropic.com"

// anthropicVersion is sent as the required "anthropic-version" header - the
// one value the Messages API documents for this shape (2023-06-01).
const anthropicVersion = "2023-06-01"

// Config is what a Client is built from.
type Config struct {
	// BaseURL is the Anthropic API's base. Empty uses defaultBaseURL.
	BaseURL string
	// APIKey is sent as the required "x-api-key" header on every request,
	// read from ANTHROPIC_API_KEY in the process environment only
	// (internal/infra/config.Config.AnthropicAPIKey) - never logged, never
	// written anywhere by this package (Client.Complete's own log line
	// names the model, token counts, latency and stop reason, never the
	// key).
	APIKey string
	// Model is the default model name sent with every request
	// (ORCHESTRA_LLM_MODEL), e.g. "claude-sonnet-5" - no date suffix
	// (params.go's own doc comment). chat.Request.Model, when set,
	// overrides it, mirroring chat.Config.Model.
	Model string
}
