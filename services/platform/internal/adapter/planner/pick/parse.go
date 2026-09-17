package pick

import (
	"regexp"
	"sort"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// candidate is one id the pick's response can name: a shortlist endpoint's
// own OperationID and Service, or one of the three built-in ids
// (service "platform", the same fixed identifier
// usecase.listCapabilities uses for the built-in tool itself).
type candidate struct {
	id      string
	service string
}

// platformService names the built-in candidates' Service, mirroring
// usecase.listCapabilities' own "platform" (internal/usecase/orchestrator.go):
// list_capabilities, propose_panel and none are not a configured service,
// so there is no contract to read a service from.
const platformService = "platform"

// candidatesFor lists every id the pick's response is matched against: the
// shortlist's own endpoints, longest id first is decided later (parse), not
// here - this just enumerates what may be found. The IDProposePanel
// candidate is included only when offerProposePanel is true - O3
// (docs/specs/offering.md), Picker.Pick's own switch, the same one
// userMessage applies to lineProposePanel (prompt.go): a response naming
// "propose_panel" cannot match a candidate that was never offered.
func candidatesFor(shortlist domain.Catalog, offerProposePanel bool) []candidate {
	candidates := make([]candidate, 0, len(shortlist.Endpoints)+builtinLineCount)

	for i := range shortlist.Endpoints {
		candidates = append(candidates, candidate{
			id:      shortlist.Endpoints[i].OperationID,
			service: shortlist.Endpoints[i].Service,
		})
	}

	candidates = append(candidates, candidate{id: IDListCapabilities, service: platformService})

	if offerProposePanel {
		candidates = append(candidates, candidate{id: IDProposePanel, service: platformService})
	}

	return append(candidates, candidate{id: IDNone, service: platformService})
}

// ambiguousPattern matches the second word e2e/narrowing/pick/client.ts's
// own parsePick looks for, case-insensitively, anywhere in the response -
// not tied to being the second token, since a model does not always answer
// with exactly the one line asked for.
var ambiguousPattern = regexp.MustCompile(`(?i)ambiguous`)

// parse turns the pick's raw response content into a usecase.Pick: the
// first candidate id that appears in content, longest id first so one id
// being a substring of another (listInventoryItems / listInventoryItem)
// cannot steal the match (docs/specs/staging.md, section 4) - the same
// rule e2e/narrowing/pick/client.ts's parsePick uses. "ambiguous" anywhere
// in content sets Ambiguous regardless of which id was found. No candidate
// id present at all is PickNone.
func parse(content string, candidates []candidate) usecase.Pick {
	ambiguous := ambiguousPattern.MatchString(content)

	byID := make(map[string]candidate, len(candidates))
	ids := make([]string, len(candidates))

	for i, c := range candidates {
		byID[c.id] = c
		ids[i] = c.id
	}

	sort.Slice(ids, func(i, j int) bool { return len(ids[i]) > len(ids[j]) })

	for _, id := range ids {
		if !strings.Contains(content, id) {
			continue
		}

		switch id {
		case IDListCapabilities:
			return usecase.Pick{Kind: usecase.PickListCapabilities, Ambiguous: ambiguous}
		case IDProposePanel:
			return usecase.Pick{Kind: usecase.PickProposePanel, Ambiguous: ambiguous}
		case IDNone:
			return usecase.Pick{Kind: usecase.PickNone, Ambiguous: ambiguous}
		default:
			c := byID[id]

			return usecase.Pick{Kind: usecase.PickOperation, Service: c.service, OperationID: id, Ambiguous: ambiguous}
		}
	}

	return usecase.Pick{Kind: usecase.PickNone, Ambiguous: ambiguous}
}
