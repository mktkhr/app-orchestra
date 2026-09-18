// app_fill.go: the fill-stage experiment's own composition
// (docs/measurements/jev-conditions.md) - the Fill config type and
// newFillOptions, called from app_staging.go's stagingOptions - split out
// of app.go for the same guard-filelen reason app_staging.go's own doc
// comment gives. The package's own doc comment is app.go's.

package app

import (
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/jev"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// Fill configures the fill-stage experiment's two independent arms
// (docs/measurements/jev-conditions.md): SkipEmpty is ORCHESTRA_FILL_SKIP_EMPTY
// (arm 1, no Jev involved at all); Enum is ORCHESTRA_FILL_ENUM (arm 2,
// "" or FillEnumNone - the default - or FillEnumJev). Both are meaningless
// without LLM.Stages == 2, the same way Picker and Gate are - staging
// never builds a fromPick fill call at all otherwise, so newFillOptions is
// only ever consulted from stagingOptions' own non-early-return path.
type Fill struct {
	// SkipEmpty is arm 1: skip the fill's model call outright for a picked
	// operation that declares no parameters and needs no request body.
	SkipEmpty bool
	// Enum is arm 2's own switch: "" or FillEnumNone (the default) skips
	// it entirely; FillEnumJev consults a jev.Filler in place of the
	// fill's model call for a picked operation whose every parameter is
	// enum-valued.
	Enum string
	// JevAPIKey is sent as internal/adapter/planner/jev's bearer token.
	// Required when Enum is FillEnumJev (ErrMissingJevAPIKey) - the same
	// key Picker.JevAPIKey, Gate.JevAPIKey and ServiceRouter.JevAPIKey
	// send.
	JevAPIKey string
	// JevBaseURL is the base URL internal/adapter/planner/jev calls,
	// mirroring Picker.JevBaseURL's own doc comment.
	JevBaseURL string
	// EnumThreshold is the confidence below which any one question's
	// answer makes jev.Filler.Fill fail open to the local fill. cmd/api
	// always sets it from config.Config.FillEnumThreshold, which already
	// defaults to 0.5 when ORCHESTRA_FILL_ENUM_THRESHOLD is unset. 0 is
	// treated by newFillOptions as "not set" - the same convention
	// Gate.JevGateThreshold's own doc comment describes - so
	// jev.NewFiller's own default (also 0.5) applies instead of a
	// threshold no real answer could ever clear.
	EnumThreshold float64
	// EnumRefusal is ORCHESTRA_FILL_ENUM_REFUSAL: "" (off, the default)
	// means neither whole-request judgement - "this operation cannot
	// answer the question at all" nor "the question is about this
	// system's capabilities in general, not about running any
	// operation" - is ever asked. FillEnumRefusalOn ("1") asks both as
	// extra options on every parameter's own Choice question
	// (jev.WithFillRefusal); FillEnumRefusalSeparate ("separate") asks
	// them instead as their own whole-request questions in the same
	// request (jev.WithFillRefusalSeparate). Only meaningful alongside
	// Enum == FillEnumJev, the same way EnumThreshold only matters there.
	EnumRefusal string
	// EnumUnsetWording is ORCHESTRA_FILL_ENUM_UNSET_WORDING: "" or
	// "narrow" (the default) keeps arm 2's original __unset__ wording;
	// "wide" additionally covers a question that explicitly asks for
	// everything on a field or removes an earlier restriction. Only
	// meaningful alongside Enum == FillEnumJev.
	EnumUnsetWording string
}

// FillEnumNone and FillEnumJev are Fill.Enum's two non-empty values,
// mirroring internal/infra/config.FillEnumNone/FillEnumJev.
const (
	FillEnumNone = "none"
	FillEnumJev  = "jev"
)

// FillEnumUnsetWordingWide is Fill.EnumUnsetWording's one non-default
// value, mirroring internal/infra/config.FillEnumUnsetWordingWide.
const FillEnumUnsetWordingWide = "wide"

// FillEnumRefusalOn and FillEnumRefusalSeparate are Fill.EnumRefusal's two
// non-default values, mirroring
// internal/infra/config.FillEnumRefusalOn/Separate.
const (
	FillEnumRefusalOn       = "1"
	FillEnumRefusalSeparate = "separate"
)

// ErrInvalidFillEnum is returned by newFillOptions when Config.Fill.Enum is
// set to anything other than "" (FillEnumNone), FillEnumNone or
// FillEnumJev - mirroring ErrInvalidGate's own defence-in-depth: cmd/api
// always goes through config.Load's own validation first
// (config.ErrInvalidFillEnum).
var ErrInvalidFillEnum = errors.New("invalid Fill.Enum, want \"\", \"none\" or \"jev\"")

// newFillOptions builds the usecase.Option list stagingOptions appends for
// the fill-stage experiment's two arms: usecase.WithFillSkipEmpty when
// cfg.Fill.SkipEmpty, usecase.WithFillEnum over a jev.Filler when
// cfg.Fill.Enum is FillEnumJev - either, both or neither, since the two
// arms are independent of each other. Both are off (nil options, no error)
// when cfg.Fill is its zero value, the default every test and caller that
// predates this subproject keeps.
func newFillOptions(cfg *Config) ([]usecase.Option, error) {
	var opts []usecase.Option

	if cfg.Fill.SkipEmpty {
		opts = append(opts, usecase.WithFillSkipEmpty())
	}

	switch cfg.Fill.Enum {
	case "", FillEnumNone:
	case FillEnumJev:
		if cfg.Fill.JevAPIKey == "" {
			return nil, ErrMissingJevAPIKey
		}

		fillerOpts := []jev.FillerOption{}
		if cfg.Fill.EnumThreshold != 0 {
			fillerOpts = append(fillerOpts, jev.WithFillThreshold(cfg.Fill.EnumThreshold))
		}

		switch cfg.Fill.EnumRefusal {
		case FillEnumRefusalOn:
			fillerOpts = append(fillerOpts, jev.WithFillRefusal())
		case FillEnumRefusalSeparate:
			fillerOpts = append(fillerOpts, jev.WithFillRefusalSeparate())
		}

		if cfg.Fill.EnumUnsetWording == FillEnumUnsetWordingWide {
			fillerOpts = append(fillerOpts, jev.WithFillUnsetWordingWide())
		}

		filler := jev.NewFiller(cfg.Fill.JevBaseURL, cfg.Fill.JevAPIKey, nil, fillerOpts...)

		opts = append(opts, usecase.WithFillEnum(filler))
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidFillEnum, cfg.Fill.Enum)
	}

	return opts, nil
}
