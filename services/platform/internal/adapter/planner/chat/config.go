// Package chat is the transport shared by every planner adapter that talks
// to a model over the OpenAI chat-completions wire format: llama.cpp behind
// llama-swap, vLLM, LM Studio and the hosted providers all speak it (D5,
// docs/specs/orchestration.md). It knows nothing about usecase.Tool or
// usecase.Decision - that mapping belongs to the planner adapters
// (internal/adapter/planner/toolcall, internal/adapter/planner/jsonmode)
// that use this package.
package chat

// Config is what a Client is built from.
type Config struct {
	// BaseURL is the OpenAI-compatible endpoint's base, e.g.
	// "http://localhost:11435/v1" (ORCHESTRA_LLM_BASE_URL). "/chat/completions"
	// is appended to it.
	BaseURL string
	// APIKey is sent as "Authorization: Bearer <APIKey>" when non-empty
	// (ORCHESTRA_LLM_API_KEY). A local runtime such as llama-swap does not
	// check it, so it may be left empty.
	APIKey string
	// Model is the default model name sent with every request
	// (ORCHESTRA_LLM_MODEL). Request.Model, when set, overrides it -
	// naming a different model is how a different backend gets used
	// behind a router such as llama-swap (D5).
	Model string
}
