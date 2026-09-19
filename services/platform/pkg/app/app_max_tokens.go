// app_max_tokens.go: the toolcall/jsonmode/pick MaxTokens option-list
// helpers newPlanner's own toolcallOptions/jsonmodeOptions (app.go) and
// app_staging.go's own pickOptions call, split out of app.go
// (harness/quality/file-length.txt's 1000-line cap - app.go was already
// at its own cap before ORCHESTRA_PLANNER_MAX_TOKENS/
// ORCHESTRA_PLANNER_PICK_MAX_TOKENS existed, the same reasoning
// app_staging.go's own doc comment gives for its own split). The
// package's own doc comment is app.go's.

package app

import (
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jsonmode"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/toolcall"
)

// toolcallMaxTokensOptions builds the toolcall.Option toolcallOptions
// appends for cfg.LLM.MaxTokens: nil when it is not positive (every test
// and caller that predates this option - toolcall.New's own default,
// chat.MaxTokens's value, applies), one toolcall.WithMaxTokens otherwise.
func toolcallMaxTokensOptions(cfg *Config) []toolcall.Option {
	if cfg.LLM.MaxTokens <= 0 {
		return nil
	}

	return []toolcall.Option{toolcall.WithMaxTokens(cfg.LLM.MaxTokens)}
}

// jsonmodeMaxTokensOptions is toolcallMaxTokensOptions' own equivalent for
// jsonmodeOptions.
func jsonmodeMaxTokensOptions(cfg *Config) []jsonmode.Option {
	if cfg.LLM.MaxTokens <= 0 {
		return nil
	}

	return []jsonmode.Option{jsonmode.WithMaxTokens(cfg.LLM.MaxTokens)}
}
