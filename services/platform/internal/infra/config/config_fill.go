// config_fill.go: the fill-stage experiment's own two arms
// (docs/measurements/jev-conditions.md), split out of config.go for
// guard-filelen (harness/quality/filelen.sh's 1000-line cap - config.go
// was already at its own cap before this subproject existed), the same
// split config_gate.go and config_service_router.go describe for their
// own variables. The package's own doc comment is config.go's.

package config

import (
	"errors"
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

// loadFill reads ORCHESTRA_FILL_SKIP_EMPTY, ORCHESTRA_FILL_ENUM and
// ORCHESTRA_FILL_ENUM_THRESHOLD and applies them to cfg, called from
// loadServiceRouter's own tail (config_service_router.go) - see that
// function's own doc comment for why it is chained there rather than
// called as its own step from config.go. ORCHESTRA_JEV_API_KEY is
// required when the fill arm is FillEnumJev (ErrMissingJevAPIKey), on top
// of - not instead of - loadPicker's, loadGate's and loadServiceRouter's
// own requiredness checks: any one of the four alone is enough to require
// the key. Both arms are independent of each other and of Picker/Gate/
// ServiceRouter, and both default to off - a deployment that sets neither
// ORCHESTRA_FILL_SKIP_EMPTY nor ORCHESTRA_FILL_ENUM starts exactly as it
// did before this subproject existed.
func loadFill(cfg *Config) error {
	cfg.FillSkipEmpty = os.Getenv("ORCHESTRA_FILL_SKIP_EMPTY") == "1"

	stage, err := loadJevStage(
		cfg, "ORCHESTRA_FILL_ENUM", FillEnumNone, FillEnumJev, ErrInvalidFillEnum,
		"ORCHESTRA_FILL_ENUM_THRESHOLD", defaultFillEnumThreshold, ErrInvalidFillEnumThreshold,
	)
	if err != nil {
		return err
	}

	cfg.FillEnum = stage.Value
	cfg.FillEnumThreshold = stage.Threshold

	return nil
}
