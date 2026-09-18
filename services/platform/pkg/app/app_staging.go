// app_staging.go: the staging subproject's own composition
// (docs/specs/staging.md) - stagingOptions, newPicker and newGate - split
// out of app.go (harness/quality/filelen.sh's 1000-line guard: app.go was
// already at its own cap once the v5 Jev trial's fan-out wiring
// (docs/measurements/jev-picker-v5.md) grew newPicker by a parameter and
// a few lines). The package's own doc comment is app.go's.

package app

import (
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/hybrid"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/pick"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// stagesTwo is the one value of Config.LLM.Stages that turns staging on
// (docs/specs/staging.md, S6) - named so the comparison below isn't a bare
// magic number (mnd, harness/quality/go/golangci.yml).
const stagesTwo = 2

// stagingOptions builds the usecase.Option list build passes to
// NewOrchestrator for the staging subproject: nil when cfg.LLM.Stages is
// not 2, or when cfg.LLM.BaseURL is empty (every test and caller that
// predates this subproject, and production with no LLM configured -
// newPlanner's own stub fallback, above), otherwise a usecase.Picker
// (newPicker) plus usecase.WithStages(2). Staging needs a real model to
// pick against; pairing it with the stub planner would send a real
// request to an empty base URL instead of exercising the stub.
func stagingOptions(cfg *Config) ([]usecase.Option, error) {
	if cfg.LLM.Stages != stagesTwo || cfg.LLM.BaseURL == "" {
		return nil, nil
	}

	// fanOut is the v5 Jev trial's own wiring
	// (docs/measurements/jev-picker-v5.md, "fan-out, not extra calls"):
	// when both the picker and the gate are jev, newPicker folds the
	// "impossible" question into the same request Pick already sends
	// (jev.WithFanOutGate) instead of this function building a second,
	// standalone Gate that would call the same API a second time for
	// every question. GateJev paired with any other picker still goes
	// through newGate/usecase.WithGate unchanged - there is no single
	// request for a non-jev picker to fold a jev gate question into.
	fanOut := cfg.Picker.Name == PickerJev && cfg.Gate.Name == GateJev

	picker, err := newPicker(cfg, fanOut)
	if err != nil {
		return nil, err
	}

	opts := []usecase.Option{usecase.WithPicker(picker), usecase.WithStages(stagesTwo)}

	if cfg.Gate.Name != "" && cfg.Gate.Name != GateNone && !fanOut {
		gate, gateErr := newGate(cfg)
		if gateErr != nil {
			return nil, gateErr
		}

		opts = append(opts, usecase.WithGate(gate))
	}

	fillOpts, err := newFillOptions(cfg)
	if err != nil {
		return nil, err
	}

	return append(opts, fillOpts...), nil
}

// PickerJev builds jev.New against
// Picker.JevBaseURL/JevAPIKey instead - a hosted picker, unrelated to
// cfg.LLM entirely. fanOut, given only by stagingOptions when
// cfg.Gate.Name is also GateJev (docs/measurements/jev-picker-v5.md),
// adds jev.WithFanOutGate(cfg.Gate.JevGateThreshold) so the one
// jev.Picker this returns folds the standalone gate question into its
// own request instead of stagingOptions ever building a second,
// standalone jev.Gate for it.
func newPicker(cfg *Config, fanOut bool) (usecase.Picker, error) {
	switch cfg.Picker.Name {
	case "", PickerLocal:
		client := chat.New(chat.Config{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model})

		return pick.New(client, cfg.LLM.Model), nil
	case PickerJev:
		if cfg.Picker.JevAPIKey == "" {
			return nil, ErrMissingJevAPIKey
		}

		opts := []jev.Option{jev.WithCriteria(cfg.Picker.JevCriteria)}
		if fanOut {
			opts = append(opts, jev.WithFanOutGate(cfg.Gate.JevGateThreshold))
		}

		if cfg.Picker.JevObjectInstructions {
			opts = append(opts, jev.WithObjectInstructions())
		}

		return jev.New(cfg.Picker.JevBaseURL, cfg.Picker.JevAPIKey, nil, opts...), nil
	case PickerHybrid:
		return newHybridPicker(cfg)
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidPicker, cfg.Picker.Name)
	}
}

// newHybridPicker builds hybrid.New over a jev.Picker and a pick.Picker,
// each built exactly as newPicker's own PickerJev/PickerLocal cases build
// theirs - except the jev half here never takes jev.WithFanOutGate: that
// option is stagingOptions' own fanOut wiring for pairing a standalone
// jev Picker with a standalone jev Gate in one request
// (docs/measurements/jev-picker-v5.md), a different mechanism from
// hybrid.Picker's own timeout/threshold fallback, and folding the two
// together is out of scope here. newPicker's own fanOut parameter is
// PickerJev-only (stagingOptions computes it from cfg.Picker.Name ==
// PickerJev), so it is already always false whenever cfg.Picker.Name is
// PickerHybrid - this function simply never looks at it.
func newHybridPicker(cfg *Config) (usecase.Picker, error) {
	if cfg.Picker.JevAPIKey == "" {
		return nil, ErrMissingJevAPIKey
	}

	jevOpts := []jev.Option{jev.WithCriteria(cfg.Picker.JevCriteria)}
	if cfg.Picker.JevObjectInstructions {
		jevOpts = append(jevOpts, jev.WithObjectInstructions())
	}

	jevPicker := jev.New(cfg.Picker.JevBaseURL, cfg.Picker.JevAPIKey, nil, jevOpts...)

	client := chat.New(chat.Config{BaseURL: cfg.LLM.BaseURL, APIKey: cfg.LLM.APIKey, Model: cfg.LLM.Model})
	localPicker := pick.New(client, cfg.LLM.Model)

	return hybrid.New(
		jevPicker, localPicker,
		hybrid.WithJevTimeout(cfg.Picker.HybridJevTimeout), hybrid.WithThreshold(cfg.Picker.HybridThreshold),
	), nil
}

// newServiceRouterOption builds the usecase.Option build passes to
// NewOrchestrator for the full-catalogue Jev trial's own follow-up
// (docs/measurements/jev-full-catalogue.md; DECISIONS.md 2026-09-18): a
// no-op when cfg.ServiceRouter.Name is "" or ServiceRouterNone - every
// test and caller that predates this subproject - otherwise
// usecase.WithServiceRouter over a jev.ServiceRouter. Unlike
// stagingOptions, this never checks cfg.LLM.Stages or cfg.LLM.BaseURL:
// Plan consults o.serviceRouter before o.narrower.Narrow regardless of
// how many calls the rest of the pipeline makes
// (internal/usecase/orchestrator.go's routeService).
func newServiceRouterOption(cfg *Config) (usecase.Option, error) {
	switch cfg.ServiceRouter.Name {
	case "", ServiceRouterNone:
		return func(*usecase.Orchestrator) {}, nil
	case ServiceRouterJev:
		if cfg.ServiceRouter.JevAPIKey == "" {
			return nil, ErrMissingJevAPIKey
		}

		opts := []jev.RouterOption{
			jev.WithRouterCriteria(cfg.ServiceRouter.JevCriteria),
			jev.WithRouterCriteriaForm(cfg.ServiceRouter.JevCriteriaForm),
		}
		router := jev.NewServiceRouter(cfg.ServiceRouter.JevBaseURL, cfg.ServiceRouter.JevAPIKey, nil, opts...)

		return usecase.WithServiceRouter(router, cfg.ServiceRouter.Threshold), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidServiceRouter, cfg.ServiceRouter.Name)
	}
}

// newGate builds the usecase.Gate stagingOptions passes to usecase.WithGate
// - the v3 Jev trial's own "noul refusal gate"
// (docs/measurements/jev-picker-v3.md). Only called by stagingOptions when
// cfg.Gate.Name is neither "" nor GateNone - that (the default; every
// test and caller that predates this subproject) skips this function
// entirely, so usecase.WithGate is never called and planStaged's gate
// step is skipped exactly as if this subproject did not exist. GateJev
// builds jev.NewGate against Gate.JevBaseURL/JevAPIKey - the same key
// Picker's own JevAPIKey sends, unrelated to which Picker is configured
// (a gate can run ahead of the local picker just as well as the jev
// one). See Gate.JevGateThreshold's own doc comment for why a zero
// threshold omits jev.WithGateThreshold rather than passing 0 through.
func newGate(cfg *Config) (usecase.Gate, error) {
	switch cfg.Gate.Name {
	case GateJev:
		if cfg.Gate.JevAPIKey == "" {
			return nil, ErrMissingJevAPIKey
		}

		opts := []jev.GateOption{}
		if cfg.Gate.JevGateThreshold != 0 {
			opts = append(opts, jev.WithGateThreshold(cfg.Gate.JevGateThreshold))
		}

		return jev.NewGate(cfg.Gate.JevBaseURL, cfg.Gate.JevAPIKey, nil, opts...), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidGate, cfg.Gate.Name)
	}
}
