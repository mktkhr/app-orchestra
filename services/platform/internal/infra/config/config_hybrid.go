// config_hybrid.go: internal/adapter/planner/hybrid's own ORCHESTRA_PICKER=hybrid
// half of this package, split out of config.go for guard-filelen
// (harness/quality/filelen.sh's 1000-line cap - config.go was already
// close to its own cap before this subproject existed), the same split
// config_gate.go describes for the v3 Jev trial's gate. The package's
// own doc comment is config.go's.

package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// ErrInvalidHybridJevTimeout is wrapped into the error returned when
// ORCHESTRA_HYBRID_JEV_TIMEOUT is set to something that does not parse as
// a time.Duration - the same reasoning ErrInvalidPlannerRepeatPenalty
// (config.go) already applies to ORCHESTRA_PLANNER_REPEAT_PENALTY.
var ErrInvalidHybridJevTimeout = errors.New("ORCHESTRA_HYBRID_JEV_TIMEOUT must be a duration such as \"800ms\"")

// ErrInvalidHybridThreshold is wrapped into the error returned when
// ORCHESTRA_HYBRID_THRESHOLD is set to something that does not parse as a
// float - the same reasoning ErrInvalidJevGateThreshold (config_gate.go)
// already applies to ORCHESTRA_JEV_GATE_THRESHOLD.
var ErrInvalidHybridThreshold = errors.New("ORCHESTRA_HYBRID_THRESHOLD must be a number")

// defaultHybridJevTimeout is used when ORCHESTRA_HYBRID_JEV_TIMEOUT is
// unset - mirrors internal/adapter/planner/hybrid's own
// defaultJevTimeout.
const defaultHybridJevTimeout = 800 * time.Millisecond

// defaultHybridThreshold is used when ORCHESTRA_HYBRID_THRESHOLD is unset
// - mirrors internal/adapter/planner/hybrid's own defaultThreshold, and
// the same number defaultJevGateThreshold (config_gate.go) already picks
// for the same evidence (docs/measurements/jev-thresholds.md).
const defaultHybridThreshold = 0.7

// parseHybridJevTimeout reads ORCHESTRA_HYBRID_JEV_TIMEOUT:
// defaultHybridJevTimeout when unset, or its time.Duration value
// otherwise.
func parseHybridJevTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultHybridJevTimeout, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidHybridJevTimeout, raw)
	}

	return value, nil
}

// parseHybridThreshold reads ORCHESTRA_HYBRID_THRESHOLD:
// defaultHybridThreshold when unset, or its float64 value otherwise.
func parseHybridThreshold(raw string) (float64, error) {
	if raw == "" {
		return defaultHybridThreshold, nil
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidHybridThreshold, raw)
	}

	return value, nil
}

// loadHybrid reads ORCHESTRA_HYBRID_JEV_TIMEOUT and
// ORCHESTRA_HYBRID_THRESHOLD and applies them to cfg, called from
// loadPickerAndGate (config.go) right after loadPicker - isolating Load
// itself from both the os.Getenv calls and their own validation (funlen,
// harness/quality/go/golangci.yml), the same reason loadGate exists.
// Both variables are read and parsed regardless of cfg.Picker - like
// JevCriteria before it, a value left set after switching Picker away
// from PickerHybrid is inert rather than an error.
func loadHybrid(cfg *Config) error {
	timeout, err := parseHybridJevTimeout(os.Getenv("ORCHESTRA_HYBRID_JEV_TIMEOUT"))
	if err != nil {
		return err
	}

	cfg.HybridJevTimeout = timeout

	threshold, err := parseHybridThreshold(os.Getenv("ORCHESTRA_HYBRID_THRESHOLD"))
	if err != nil {
		return err
	}

	cfg.HybridThreshold = threshold

	return nil
}
