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

// parseNoneOrJev is the shared "off/on" shape ORCHESTRA_GATE and
// ORCHESTRA_SERVICE_ROUTER (config_service_router.go) both parse: raw
// empty means none, raw already being none or jev passes through
// unchanged, anything else is errInvalid wrapping raw. Factored out once
// dupl (harness/quality/go/golangci.yml, threshold 100) flagged the two
// ad hoc switches as near-identical clones.
func parseNoneOrJev(raw, none, jev string, errInvalid error) (string, error) {
	switch raw {
	case "":
		return none, nil
	case none, jev:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", errInvalid, raw)
	}
}

// parseFloatOrDefault is the shared shape ORCHESTRA_JEV_GATE_THRESHOLD and
// ORCHESTRA_SERVICE_ROUTER_THRESHOLD both parse: def when raw is empty,
// otherwise raw's float64 value, or errInvalid wrapping raw when it fails
// to parse. See parseNoneOrJev's own doc comment for why this is shared.
func parseFloatOrDefault(raw string, def float64, errInvalid error) (float64, error) {
	if raw == "" {
		return def, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errInvalid, raw)
	}

	return value, nil
}

// jevStage is loadJevStage's own return shape: a struct, not two bare
// return values, so gocritic's unnamedResult (harness/quality/go/golangci.yml)
// has nothing to flag and the caller reads .Value/.Threshold by name
// rather than by position.
type jevStage struct {
	Value     string
	Threshold float64
}

// loadJevStage reads envVar (parseNoneOrJev, none/jev), requiring
// cfg.JevAPIKey when it reads jev (ErrMissingJevAPIKey), then
// thresholdEnvVar (parseFloatOrDefault) - the shared shape loadGate and
// loadServiceRouter (config_service_router.go) both have. Neither field
// is applied to cfg here: each caller does that itself, so a caller can
// decide in which order its own two fields are set (not that either
// order matters today) without this helper reaching back into a specific
// pair of Config fields by name.
func loadJevStage(
	cfg *Config, envVar, none, jev string, errInvalid error,
	thresholdEnvVar string, defaultThreshold float64, errInvalidThreshold error,
) (jevStage, error) {
	value, err := parseNoneOrJev(os.Getenv(envVar), none, jev, errInvalid)
	if err != nil {
		return jevStage{}, err
	}

	if value == jev && cfg.JevAPIKey == "" {
		return jevStage{}, ErrMissingJevAPIKey
	}

	threshold, err := parseFloatOrDefault(os.Getenv(thresholdEnvVar), defaultThreshold, errInvalidThreshold)
	if err != nil {
		return jevStage{}, err
	}

	return jevStage{Value: value, Threshold: threshold}, nil
}

// loadGate reads ORCHESTRA_GATE and ORCHESTRA_JEV_GATE_THRESHOLD and
// applies them to cfg (loadJevStage), called from Load right after
// loadPicker (config.go) - which must run first, since loadJevStage's own
// ORCHESTRA_JEV_API_KEY requiredness check reads cfg.JevAPIKey, which
// loadPicker is what sets. ORCHESTRA_JEV_API_KEY is required when the
// gate is GateJev (ErrMissingJevAPIKey), on top of - not instead of -
// loadPicker's own requiredness check for PickerJev: either one alone is
// enough to require the key.
func loadGate(cfg *Config) error {
	stage, err := loadJevStage(
		cfg, "ORCHESTRA_GATE", GateNone, GateJev, ErrInvalidGate,
		"ORCHESTRA_JEV_GATE_THRESHOLD", defaultJevGateThreshold, ErrInvalidJevGateThreshold,
	)
	if err != nil {
		return err
	}

	cfg.Gate = stage.Value
	cfg.JevGateThreshold = stage.Threshold

	// loadServiceRouter (config_service_router.go) is chained onto the
	// tail of this function, rather than called as its own step from
	// Load/loadPickerAndGate (config.go), only to keep config.go - already
	// at its own 1000-line cap (harness/quality/file-length.txt) - from
	// growing by even one more call site.
	return loadServiceRouter(cfg)
}
