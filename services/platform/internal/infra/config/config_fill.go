// config_fill.go: the fill-stage experiment's own two arms
// (docs/measurements/jev-conditions.md), split out of config.go for
// guard-filelen (harness/quality/filelen.sh's 1000-line cap - config.go
// was already at its own cap before this subproject existed), the same
// split config_gate.go and config_service_router.go describe for their
// own variables. The package's own doc comment is config.go's.

package config

import (
	"errors"
	"fmt"
	"os"
)

// ErrInvalidFillEnum is wrapped into the error returned when
// ORCHESTRA_FILL_ENUM names anything other than FillEnumNone or
// FillEnumJev - the same reasoning ErrInvalidGate (config_gate.go) already
// applies to ORCHESTRA_GATE.
var ErrInvalidFillEnum = errors.New("ORCHESTRA_FILL_ENUM must be none or jev")

// ErrInvalidFillEnumThreshold is wrapped into the error returned when
// ORCHESTRA_FILL_ENUM_THRESHOLD is set to something that does not parse as
// a float - the same reasoning ErrInvalidJevGateThreshold (config_gate.go)
// already applies to ORCHESTRA_JEV_GATE_THRESHOLD.
var ErrInvalidFillEnumThreshold = errors.New("ORCHESTRA_FILL_ENUM_THRESHOLD must be a number")

// FillEnumNone and FillEnumJev are ORCHESTRA_FILL_ENUM's two accepted
// values: no arm 2 at all (byte-identical to today, the default) and
// internal/adapter/planner/jev's enum-answer Filler, respectively.
const (
	FillEnumNone = "none"
	FillEnumJev  = "jev"
)

// defaultFillEnumThreshold is used when ORCHESTRA_FILL_ENUM_THRESHOLD is
// unset - the same starting point defaultJevGateThreshold's neighbours
// (defaultAmbiguityThreshold, internal/adapter/planner/jev) use for a
// confidence gate with no measurement of its own yet to justify a
// different number.
const defaultFillEnumThreshold = 0.5

// ErrInvalidFillEnumUnsetWording is wrapped into the error returned when
// ORCHESTRA_FILL_ENUM_UNSET_WORDING is set to something other than
// FillEnumUnsetWordingNarrow or FillEnumUnsetWordingWide - the same
// reasoning ErrInvalidServiceRouterCriteria (config_service_router.go)
// already applies to ORCHESTRA_SERVICE_ROUTER_CRITERIA.
var ErrInvalidFillEnumUnsetWording = errors.New("ORCHESTRA_FILL_ENUM_UNSET_WORDING must be narrow or wide")

// FillEnumUnsetWordingNarrow and FillEnumUnsetWordingWide are
// ORCHESTRA_FILL_ENUM_UNSET_WORDING's two accepted values:
// FillEnumUnsetWordingNarrow is arm 2's original __unset__ criterion,
// unchanged since it was introduced - the default, so a deployment that
// never sets this variable sends exactly what it always has - and
// FillEnumUnsetWordingWide additionally covers a question that explicitly
// asks for everything or removes an earlier restriction (today's
// dialogue d06 turn 2 regression, docs/measurements/jev-conditions.md).
const (
	FillEnumUnsetWordingNarrow = "narrow"
	FillEnumUnsetWordingWide   = "wide"
)

// defaultFillEnumUnsetWording is used when
// ORCHESTRA_FILL_ENUM_UNSET_WORDING is unset.
const defaultFillEnumUnsetWording = FillEnumUnsetWordingNarrow

// parseFillEnumUnsetWording reads ORCHESTRA_FILL_ENUM_UNSET_WORDING:
// defaultFillEnumUnsetWording when unset, or exactly
// FillEnumUnsetWordingNarrow or FillEnumUnsetWordingWide otherwise - the
// same reasoning parseServiceRouterCriteria (config_service_router.go)
// already applies to ORCHESTRA_SERVICE_ROUTER_CRITERIA.
func parseFillEnumUnsetWording(raw string) (string, error) {
	switch raw {
	case "":
		return defaultFillEnumUnsetWording, nil
	case FillEnumUnsetWordingNarrow, FillEnumUnsetWordingWide:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidFillEnumUnsetWording, raw)
	}
}

// loadFill reads ORCHESTRA_FILL_SKIP_EMPTY, ORCHESTRA_FILL_ENUM,
// ORCHESTRA_FILL_ENUM_THRESHOLD, ORCHESTRA_FILL_ENUM_REFUSAL and
// ORCHESTRA_FILL_ENUM_UNSET_WORDING and applies them to cfg, called from
// loadServiceRouter's own tail (config_service_router.go) - see that
// function's own doc comment for why it is chained there rather than
// called as its own step from config.go. ORCHESTRA_JEV_API_KEY is
// required when the fill arm is FillEnumJev (ErrMissingJevAPIKey), on top
// of - not instead of - loadPicker's, loadGate's and loadServiceRouter's
// own requiredness checks: any one of the four alone is enough to require
// the key. All four options are independent of each other and of Picker/
// Gate/ServiceRouter, and all default to off/unchanged - a deployment
// that sets none of them starts exactly as it did before this subproject
// existed. ORCHESTRA_FILL_ENUM_REFUSAL and ORCHESTRA_FILL_ENUM_UNSET_WORDING
// are read regardless of ORCHESTRA_FILL_ENUM's own value, the same way
// ORCHESTRA_FILL_ENUM_THRESHOLD already is - they are only ever consulted
// by internal/adapter/planner/jev's own Filler, which is only ever built
// when ORCHESTRA_FILL_ENUM is FillEnumJev (pkg/app/app_fill.go).
func loadFill(cfg *Config) error {
	cfg.FillSkipEmpty = os.Getenv("ORCHESTRA_FILL_SKIP_EMPTY") == "1"
	cfg.FillEnumRefusal = os.Getenv("ORCHESTRA_FILL_ENUM_REFUSAL") == "1"

	stage, err := loadJevStage(
		cfg, "ORCHESTRA_FILL_ENUM", FillEnumNone, FillEnumJev, ErrInvalidFillEnum,
		"ORCHESTRA_FILL_ENUM_THRESHOLD", defaultFillEnumThreshold, ErrInvalidFillEnumThreshold,
	)
	if err != nil {
		return err
	}

	cfg.FillEnum = stage.Value
	cfg.FillEnumThreshold = stage.Threshold

	unsetWording, err := parseFillEnumUnsetWording(os.Getenv("ORCHESTRA_FILL_ENUM_UNSET_WORDING"))
	if err != nil {
		return err
	}

	cfg.FillEnumUnsetWording = unsetWording

	return nil
}
