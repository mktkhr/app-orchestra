// config_pick_wording.go: ORCHESTRA_PICK_WORDING, split out of config.go
// the same way config_max_tokens.go and config_hybrid.go are split out -
// config.go was already close to its own 1000-line cap (guard-filelen,
// harness/quality/filelen.sh) before this variable existed. The package's
// own doc comment is config.go's.
//
// The pick stage's three built-in candidate lines (list_capabilities,
// propose_panel, none - internal/adapter/planner/pick) carry Japanese
// phrasing tuned for qwen3.5-9b-q8 (34/34 on ORCHESTRA_PLANNER_STAGES=2
// make eval). Measured 2026-09-19, that same phrasing reads bonsai2-27b
// (a ternary 27B served through llama-swap) at 30/34, three of its four
// losses the same shape - so the wording is model-specific and the tree
// needs to be able to carry more than one named set, exactly the shape
// ORCHESTRA_PLANNER_WORDING already gives the toolcall planner's own
// words (parsePlannerWording, above).

package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
)

// loadPickWording reads ORCHESTRA_PICK_WORDING and applies it to
// cfg.PickWording - called from loadPicker's own tail (config.go), the
// same "chained onto the end of the function that already owns this
// area's other env vars" shape loadPlannerMaxTokens is chained onto
// loadFill for. Config.PickWording selects the pick stage's own named set
// of built-in-line phrasing
// (internal/adapter/planner/pick.WordingByName). Defaults to
// pick.DefaultWording().Name ("v1") when unset - with it unset, every
// pick request is byte-identical to today's. Ignored when PlannerStages
// is not 2 - Config.Picker's own doc comment gives the same reasoning.
func loadPickWording(cfg *Config) error {
	pickWording, err := parsePickWording(os.Getenv("ORCHESTRA_PICK_WORDING"))
	if err != nil {
		return err
	}

	cfg.PickWording = pickWording

	return nil
}

// ErrInvalidPickWording is wrapped into the error returned when
// ORCHESTRA_PICK_WORDING names anything other than pick.WordingNames(). An
// unrecognised name fails startup rather than silently falling back to
// pick.DefaultWording() - the same reasoning ErrInvalidPlannerWording
// already applies to ORCHESTRA_PLANNER_WORDING.
var ErrInvalidPickWording = errors.New("invalid ORCHESTRA_PICK_WORDING")

// parsePickWording reads ORCHESTRA_PICK_WORDING: pick.DefaultWording().Name
// ("v1") when unset, or exactly one of pick.WordingNames() otherwise - see
// ErrInvalidPickWording for why anything else fails startup instead of
// falling back to the default. The error names every known set, so an
// operator who mistypes one sees the full list rather than having to go
// read the source.
func parsePickWording(raw string) (string, error) {
	if raw == "" {
		return pick.DefaultWording().Name, nil
	}

	if _, ok := pick.WordingByName(raw); !ok {
		return "", fmt.Errorf("%w: %q, want one of %s", ErrInvalidPickWording, raw, strings.Join(pick.WordingNames(), ", "))
	}

	return raw, nil
}
