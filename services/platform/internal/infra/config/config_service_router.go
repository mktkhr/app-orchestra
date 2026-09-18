// config_service_router.go: the full-catalogue Jev trial's own follow-up
// (docs/measurements/jev-full-catalogue.md; DECISIONS.md 2026-09-18) half
// of this package, split out of config.go for guard-filelen
// (harness/quality/filelen.sh's 1000-line cap), the same split config_gate.go
// already describes for that subproject. The package's own doc comment is
// config.go's.

package config

import (
	"errors"
	"fmt"
	"os"
)

// ErrInvalidServiceRouter is wrapped into the error returned when
// ORCHESTRA_SERVICE_ROUTER names anything other than ServiceRouterNone or
// ServiceRouterJev - the same reasoning ErrInvalidGate (config_gate.go)
// already applies to ORCHESTRA_GATE.
var ErrInvalidServiceRouter = errors.New("ORCHESTRA_SERVICE_ROUTER must be none or jev")

// ErrInvalidServiceRouterThreshold is wrapped into the error returned when
// ORCHESTRA_SERVICE_ROUTER_THRESHOLD is set to something that does not
// parse as a float - the same reasoning ErrInvalidJevGateThreshold
// (config_gate.go) already applies to ORCHESTRA_JEV_GATE_THRESHOLD.
var ErrInvalidServiceRouterThreshold = errors.New("ORCHESTRA_SERVICE_ROUTER_THRESHOLD must be a number")

// ServiceRouterNone and ServiceRouterJev are ORCHESTRA_SERVICE_ROUTER's
// two accepted values: no router at all (Plan goes straight from
// catalogFor to o.narrower.Narrow, byte for byte as before this
// subproject existed) and internal/adapter/planner/jev's own
// ServiceRouter, respectively. ServiceRouterNone is the default.
const (
	ServiceRouterNone = "none"
	ServiceRouterJev  = "jev"
)

// defaultServiceRouterThreshold is used when
// ORCHESTRA_SERVICE_ROUTER_THRESHOLD is unset - the same 0.5 default
// internal/adapter/planner/jev's own defaultAmbiguityThreshold (the
// picker's own confidence threshold) uses, chosen because the
// full-catalogue round's own measured service probabilities
// (docs/measurements/jev-full-catalogue.md) ran a mean of 0.765 and a
// median of 0.845 - a router that is even roughly sure clears this by a
// wide margin, so 0.5 mainly screens out the genuinely split cases
// rather than second-guessing a confident answer.
const defaultServiceRouterThreshold = 0.5

// ErrInvalidServiceRouterCriteria is wrapped into the error returned when
// ORCHESTRA_SERVICE_ROUTER_CRITERIA is set to something other than
// ServiceRouterCriteriaNames or ServiceRouterCriteriaOps - the same
// reasoning ErrInvalidJevCriteria (config.go) already applies to
// ORCHESTRA_JEV_CRITERIA.
var ErrInvalidServiceRouterCriteria = errors.New("ORCHESTRA_SERVICE_ROUTER_CRITERIA must be names or ops")

// ServiceRouterCriteriaNames and ServiceRouterCriteriaOps are
// ORCHESTRA_SERVICE_ROUTER_CRITERIA's two accepted values:
// ServiceRouterCriteriaNames is internal/adapter/planner/jev's original
// per-service criterion (a handful of that service's own operation
// names) - the default, so a deployment that never sets this variable
// sends exactly what it always has - and ServiceRouterCriteriaOps is the
// full-catalogue Jev trial's own follow-up (docs/measurements/
// jev-full-catalogue.md; DECISIONS.md 2026-09-18): every one of that
// service's own operations, not just a handful.
const (
	ServiceRouterCriteriaNames = "names"
	ServiceRouterCriteriaOps   = "ops"
)

// defaultServiceRouterCriteria is used when ORCHESTRA_SERVICE_ROUTER_CRITERIA
// is unset.
const defaultServiceRouterCriteria = ServiceRouterCriteriaNames

// parseServiceRouterCriteria reads ORCHESTRA_SERVICE_ROUTER_CRITERIA:
// defaultServiceRouterCriteria when unset, or exactly
// ServiceRouterCriteriaNames or ServiceRouterCriteriaOps otherwise - the
// same reasoning parseJevCriteria (config.go) already applies to
// ORCHESTRA_JEV_CRITERIA.
func parseServiceRouterCriteria(raw string) (string, error) {
	switch raw {
	case "":
		return defaultServiceRouterCriteria, nil
	case ServiceRouterCriteriaNames, ServiceRouterCriteriaOps:
		return raw, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidServiceRouterCriteria, raw)
	}
}

// loadServiceRouter reads ORCHESTRA_SERVICE_ROUTER,
// ORCHESTRA_SERVICE_ROUTER_THRESHOLD and ORCHESTRA_SERVICE_ROUTER_CRITERIA
// and applies them to cfg (loadJevStage, config_gate.go), called from
// loadGate's own tail - see that function's own doc comment for why it is
// chained there rather than called as its own step from config.go.
// ORCHESTRA_JEV_API_KEY is required when the router is ServiceRouterJev
// (ErrMissingJevAPIKey), on top of - not instead of - loadPicker's own and
// loadGate's own requiredness checks: any one of the three alone is
// enough to require the key.
func loadServiceRouter(cfg *Config) error {
	stage, err := loadJevStage(
		cfg, "ORCHESTRA_SERVICE_ROUTER", ServiceRouterNone, ServiceRouterJev, ErrInvalidServiceRouter,
		"ORCHESTRA_SERVICE_ROUTER_THRESHOLD", defaultServiceRouterThreshold, ErrInvalidServiceRouterThreshold,
	)
	if err != nil {
		return err
	}

	cfg.ServiceRouter = stage.Value
	cfg.ServiceRouterThreshold = stage.Threshold

	criteria, err := parseServiceRouterCriteria(os.Getenv("ORCHESTRA_SERVICE_ROUTER_CRITERIA"))
	if err != nil {
		return err
	}

	cfg.ServiceRouterCriteria = criteria

	// loadFill (config_fill.go) is chained onto this function's own tail,
	// rather than called as its own step from Load/loadPickerAndGate
	// (config.go), for the same reason loadGate's own tail calls this
	// function instead: config.go is already at its own 1000-line cap
	// (harness/quality/file-length.txt).
	return loadFill(cfg)
}
