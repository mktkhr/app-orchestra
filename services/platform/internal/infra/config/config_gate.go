// config_gate.go: the v3 Jev trial's own "noul refusal gate"
// (docs/measurements/jev-picker-v3.md) half of this package, split out
// of config.go for guard-filelen (harness/quality/filelen.sh's 1000-line
// cap - config.go was already at its own cap before this subproject
// existed), the same split app_picker_test.go describes for pkg/app's
// test files. The package's own doc comment is config.go's.

package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// ErrInvalidGate is wrapped into the error returned when ORCHESTRA_GATE
// names anything other than GateNone or GateJev - the same reasoning
// ErrInvalidPicker (config.go) already applies to ORCHESTRA_PICKER.
var ErrInvalidGate = errors.New("ORCHESTRA_GATE must be none or jev")

// ErrInvalidJevGateThreshold is wrapped into the error returned when
// ORCHESTRA_JEV_GATE_THRESHOLD is set to something that does not parse as
// a float - the same reasoning ErrInvalidPlannerRepeatPenalty (config.go)
// already applies to ORCHESTRA_PLANNER_REPEAT_PENALTY.
var ErrInvalidJevGateThreshold = errors.New("ORCHESTRA_JEV_GATE_THRESHOLD must be a number")

// GateNone and GateJev are ORCHESTRA_GATE's two accepted values: no gate
// (planStaged goes straight from idAffinity to the pick, byte for byte as
// before this subproject existed) and internal/adapter/planner/jev's
// "noul refusal gate", respectively. GateNone is the default.
const (
	GateNone = "none"
	GateJev  = "jev"
)

// defaultJevGateThreshold is used when ORCHESTRA_JEV_GATE_THRESHOLD is
// unset - the v3 trial's own starting point, mirrored by
// internal/adapter/planner/jev's own defaultGateThreshold.
const defaultJevGateThreshold = 0.7

// parseGate reads ORCHESTRA_GATE: GateNone when unset, or exactly
// GateNone or GateJev otherwise - the same reasoning parsePicker
// (config.go) already applies to ORCHESTRA_PICKER.
func parseGate(raw string) (string, error) {
	switch raw {
	case "":
		return GateNone, nil
	case GateNone, GateJev:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidGate, raw)
	}
}

// parseJevGateThreshold reads ORCHESTRA_JEV_GATE_THRESHOLD:
// defaultJevGateThreshold when unset, or its float64 value otherwise -
// the same parse-or-default shape parsePlannerRepeatPenalty (config.go)
// gives ORCHESTRA_PLANNER_REPEAT_PENALTY, except this field is never
// optional (the gate always has a threshold, set or defaulted) so this
// returns a plain float64 rather than a "was it set" wrapper struct.
func parseJevGateThreshold(raw string) (float64, error) {
	if raw == "" {
		return defaultJevGateThreshold, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidJevGateThreshold, raw)
	}

	return value, nil
}

// loadGate reads ORCHESTRA_GATE and ORCHESTRA_JEV_GATE_THRESHOLD and
// applies them to cfg, called from Load right after loadPicker
// (config.go) - which must run first, since this function's own
// ORCHESTRA_JEV_API_KEY requiredness check reads cfg.JevAPIKey, which
// loadPicker is what sets. ORCHESTRA_JEV_API_KEY is required when the
// gate is GateJev (ErrMissingJevAPIKey), on top of - not instead of -
// loadPicker's own requiredness check for PickerJev: either one alone is
// enough to require the key.
func loadGate(cfg *Config) error {
	gate, err := parseGate(os.Getenv("ORCHESTRA_GATE"))
	if err != nil {
		return err
	}

	if gate == GateJev && cfg.JevAPIKey == "" {
		return ErrMissingJevAPIKey
	}

	cfg.Gate = gate

	threshold, err := parseJevGateThreshold(os.Getenv("ORCHESTRA_JEV_GATE_THRESHOLD"))
	if err != nil {
		return err
	}

	cfg.JevGateThreshold = threshold

	return nil
}
