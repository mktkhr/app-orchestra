// config_max_tokens.go: ORCHESTRA_PLANNER_MAX_TOKENS and
// ORCHESTRA_PLANNER_PICK_MAX_TOKENS, split out of config.go for
// guard-filelen (harness/quality/filelen.sh's 1000-line cap - config.go
// was already at its own cap before these two variables existed), the
// same split config_fill.go and config_hybrid.go describe for their own
// variables. The package's own doc comment is config.go's.
//
// The two budgets were a single fixed chat.Request.MaxTokens each
// (internal/adapter/planner/chat.MaxTokens's own 1024, and
// internal/adapter/planner/pick's own 200) until measured 2026-09-19: on
// claude-sonnet-5 with thinking on, 34 of 176 calls stopped at the fixed
// fill budget and the corpus score fell 81 -> 56 - the budget, not the
// model, had failed. These two variables override each default without
// changing it: unset, Config.PlannerMaxTokens/PlannerPickMaxTokens are
// byte-identical to the fixed values they replace.

package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// defaultPlannerMaxTokens is used when ORCHESTRA_PLANNER_MAX_TOKENS is
// unset - the toolcall/jsonmode planners' fill budget, byte-identical to
// the fixed value internal/adapter/planner/chat.MaxTokens used to return
// before it became configurable.
const defaultPlannerMaxTokens = 1024

// defaultPlannerPickMaxTokens is used when ORCHESTRA_PLANNER_PICK_MAX_TOKENS
// is unset - the pick stage's own budget, byte-identical to the fixed
// pickMaxTokens internal/adapter/planner/pick used to return before it
// became configurable (AC-S-104, docs/specs/staging.md section 4).
const defaultPlannerPickMaxTokens = 200

// ErrInvalidPlannerMaxTokens is returned when ORCHESTRA_PLANNER_MAX_TOKENS
// is set to something other than a positive integer, mirroring
// ErrInvalidContextTurns's own refusal to accept a non-positive budget.
var ErrInvalidPlannerMaxTokens = errors.New("ORCHESTRA_PLANNER_MAX_TOKENS must be a positive integer")

// ErrInvalidPlannerPickMaxTokens is returned when
// ORCHESTRA_PLANNER_PICK_MAX_TOKENS is set to something other than a
// positive integer, the same reasoning ErrInvalidPlannerMaxTokens gives
// for the fill budget.
var ErrInvalidPlannerPickMaxTokens = errors.New("ORCHESTRA_PLANNER_PICK_MAX_TOKENS must be a positive integer")

// loadPlannerMaxTokens reads ORCHESTRA_PLANNER_MAX_TOKENS and
// ORCHESTRA_PLANNER_PICK_MAX_TOKENS and applies them to cfg, called from
// loadFill's own tail (config_fill.go) - see that function's own doc
// comment for why it is chained there rather than called as its own step
// from Load/loadLLM.
func loadPlannerMaxTokens(cfg *Config) error {
	maxTokens, err := parsePlannerMaxTokens(os.Getenv("ORCHESTRA_PLANNER_MAX_TOKENS"))
	if err != nil {
		return err
	}

	cfg.PlannerMaxTokens = maxTokens

	pickMaxTokens, err := parsePlannerPickMaxTokens(os.Getenv("ORCHESTRA_PLANNER_PICK_MAX_TOKENS"))
	if err != nil {
		return err
	}

	cfg.PlannerPickMaxTokens = pickMaxTokens

	return nil
}

// parsePlannerMaxTokens reads ORCHESTRA_PLANNER_MAX_TOKENS:
// defaultPlannerMaxTokens when unset or empty, or the positive integer it
// names otherwise. See ErrInvalidPlannerMaxTokens for why anything else
// fails startup instead of falling back to the default - the same shape
// parseContextTurns (config.go) already gives ORCHESTRA_CONTEXT_TURNS.
func parsePlannerMaxTokens(raw string) (int, error) {
	if raw == "" {
		return defaultPlannerMaxTokens, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPlannerMaxTokens, raw)
	}

	return n, nil
}

// parsePlannerPickMaxTokens reads ORCHESTRA_PLANNER_PICK_MAX_TOKENS:
// defaultPlannerPickMaxTokens when unset or empty, or the positive integer
// it names otherwise - the same shape parsePlannerMaxTokens gives the fill
// budget.
func parsePlannerPickMaxTokens(raw string) (int, error) {
	if raw == "" {
		return defaultPlannerPickMaxTokens, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPlannerPickMaxTokens, raw)
	}

	return n, nil
}
