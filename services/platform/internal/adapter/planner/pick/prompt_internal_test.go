package pick

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// userMessageCatalog is one shortlist entry, just enough to exercise
// userMessage's framing without restating shortlistCatalog (picker_test.go,
// package pick_test - unreachable from here, since userMessage itself is
// unexported and this file is a white-box test in package pick).
func userMessageCatalog() domain.Catalog {
	return domain.Catalog{Endpoints: []domain.Endpoint{
		{
			Service:            "inventory",
			OperationID:        "listInventoryItems",
			ServiceDisplayName: "在庫管理",
			Summary:            "在庫の一覧を返す",
		},
	}}
}

// TestUserMessageWithNoAnswersIsByteIdenticalToBeforeAnswersExisted is the
// regression guard docs/specs/staging.md section 4 (added 2026-09-16
// alongside the ask_user degradation fix) calls for: AC-S-103's own
// measurement, and the stages2 comparison it feeds, depend on the pick
// seeing exactly the same prompt as before whenever there is nothing new to
// tell it - so an empty (or nil) answers must never add so much as one
// byte.
func TestUserMessageWithNoAnswersIsByteIdenticalToBeforeAnswersExisted(t *testing.T) {
	catalog := userMessageCatalog()

	want := "質問: 在庫を見せて\n\n候補:\n" +
		"listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
		lineListCapabilities + "\n" + lineProposePanel + "\n" + lineNone

	assert.Equal(t, want, userMessage("在庫を見せて", nil, catalog))
	assert.Equal(t, want, userMessage("在庫を見せて", []usecase.Answer{}, catalog),
		"an empty, non-nil answers slice must build the same message as nil")
}

// TestUserMessageWithAnswersAddsOneLinePerAnswerBeforeTheCandidates is the
// non-empty case docs/specs/staging.md section 4 adds: one "回答:
// <param>=<value>" line per answer, in order, after the question line and
// before the blank line and 候補: line - so a re-plan following a rule 2
// ask (askDegrade, internal/usecase/orchestrator_ask.go) tells the pick
// what the person already answered, not just the original query.
func TestUserMessageWithAnswersAddsOneLinePerAnswerBeforeTheCandidates(t *testing.T) {
	catalog := userMessageCatalog()

	answers := []usecase.Answer{{Param: "kind", Value: "sales"}, {Param: "status", Value: "open"}}

	want := "質問: 注文を見たい\n" +
		"回答: kind=sales\n" +
		"回答: status=open\n" +
		"\n候補:\n" +
		"listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
		lineListCapabilities + "\n" + lineProposePanel + "\n" + lineNone

	assert.Equal(t, want, userMessage("注文を見たい", answers, catalog))
}
