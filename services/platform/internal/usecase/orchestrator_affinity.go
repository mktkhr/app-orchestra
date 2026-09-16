package usecase

import (
	"context"
	"log/slog"
	"regexp"
	"slices"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

// idTokenPattern matches a maximal run of characters an id is made of -
// letters, digits, underscore, hyphen - the same alphabet idAffinity
// extracts from the question before testing any endpoint's own Pattern
// against it. Fixed, unlike a contract's own Pattern (compiled per call by
// compiledPattern below), so it is compiled once at package init, the same
// idiom internal/adapter/planner/pick/parse.go's own ambiguousPattern
// uses.
var idTokenPattern = regexp.MustCompile(`[A-Za-z0-9_-]+`)

// compiledPattern compiles pattern, logging and returning ok=false rather
// than failing or panicking when it cannot: a contract's Pattern is
// operator-authored text idAffinity cannot validate ahead of time, so a
// malformed one (an unbalanced bracket, say) must be a defect the operator
// can see, not a request that fails or a process that dies
// (regexp.Compile, never MustCompile). Not cached - a catalogue is small
// and planStaged runs at most once per request, so the cost of recompiling
// a handful of patterns is not worth the mutable global state a cache
// would add (harness/quality/go/golangci.yml, gochecknoglobals).
func compiledPattern(ctx context.Context, pattern string) (*regexp.Regexp, bool) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		slog.Default().WarnContext(ctx, "invalid id pattern in catalogue, ignored",
			slog.String("pattern", pattern), slog.Any("error", err))

		return nil, false
	}

	return re, true
}

// idAffinity scans query for a token matching some endpoint parameter's
// declared id Pattern and reports the single service every matching
// endpoint belongs to (TODO.md, "real-attendance-detail"; DECISIONS.md,
// 2026-09-17, "real-catalogue eval"): 「att-002の内容」matches attendance's
// GetAttendanceRecord `^att-[0-9]+$` and nothing in inventory, so the pick
// should only ever be offered attendance's endpoints for it.
//
// The second return is false when no token matches any endpoint's
// Pattern, or when the matching endpoints span two or more services - an
// id shape shared by two services (or found nowhere) is not evidence for
// either, so planStaged leaves the shortlist exactly as it found it. Only
// catalog's own request parameters are scanned, not a response schema's
// id property: the pick chooses among endpoints by what a person can
// supply on the way in, not by what comes back.
func idAffinity(ctx context.Context, query string, catalog domain.Catalog) (string, bool) {
	tokens := idTokenPattern.FindAllString(query, -1)
	if len(tokens) == 0 {
		return "", false
	}

	services := make(map[string]struct{}, 1)

	for e := range catalog.Endpoints {
		endpoint := &catalog.Endpoints[e]

		for p := range endpoint.Parameters {
			pattern := endpoint.Parameters[p].Schema.Pattern
			if pattern == "" {
				continue
			}

			re, ok := compiledPattern(ctx, pattern)
			if !ok {
				continue
			}

			if slices.ContainsFunc(tokens, re.MatchString) {
				services[endpoint.Service] = struct{}{}

				break
			}
		}
	}

	if len(services) != 1 {
		return "", false
	}

	for svc := range services {
		return svc, true
	}

	return "", false
}

// narrowToService returns the subset of catalog's endpoints belonging to
// service, in the same order catalog already held them - the shortlist's
// own ranking (docs/specs/shortlisting.md), which idAffinity's narrowing
// must not disturb.
func narrowToService(catalog domain.Catalog, service string) domain.Catalog {
	endpoints := make([]domain.Endpoint, 0, len(catalog.Endpoints))

	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].Service == service {
			endpoints = append(endpoints, catalog.Endpoints[i])
		}
	}

	return domain.Catalog{Endpoints: endpoints}
}
