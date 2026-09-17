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

// TestUserMessageWithNoAnswersOrTurnsIsByteIdenticalToBeforeTheyExisted is
// the regression guard docs/specs/staging.md section 4 calls for (added
// 2026-09-16 alongside the ask_user degradation fix, extended 2026-09-17
// when turns joined it): AC-S-103's own measurement, and the stages2
// comparison it feeds, depend on the pick seeing exactly the same prompt as
// before whenever there is nothing new to tell it - so empty (or nil)
// answers and turns must never add so much as one byte.
func TestUserMessageWithNoAnswersOrTurnsIsByteIdenticalToBeforeTheyExisted(t *testing.T) {
	catalog := userMessageCatalog()

	want := "質問: 在庫を見せて\n\n候補:\n" +
		"listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
		lineListCapabilities + "\n" + lineProposePanel + "\n" + lineNone

	assert.Equal(t, want, userMessage("在庫を見せて", nil, nil, catalog))
	assert.Equal(t, want, userMessage("在庫を見せて", []usecase.Answer{}, []usecase.Turn{}, catalog),
		"empty, non-nil answers and turns slices must build the same message as nil")
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

	assert.Equal(t, want, userMessage("注文を見たい", answers, nil, catalog))
}

// TestUserMessageWithTurnsAddsOneLinePerTurnAfterAnswers is the with-turns
// case docs/measurements/jev-v5.md's isolation result adds (2026-09-17):
// one "直前: <serviceDisplayName> / <operation display name>（<question>）"
// line per turn, oldest first, after any 回答 lines and before the blank
// line and 候補: line. The second turn's operation ("listAttendances") is
// not in the shortlist, so its display names fall back to the turn's own
// raw Service/OperationID (turnLine's documented fallback).
func TestUserMessageWithTurnsAddsOneLinePerTurnAfterAnswers(t *testing.T) {
	catalog := userMessageCatalog()

	turns := []usecase.Turn{
		{
			Question: "在庫の一覧を見せて", Kind: usecase.ResultKindResult,
			Service: "inventory", OperationID: "listInventoryItems",
		},
		{
			Question: "勤怠の方も見せて", Kind: usecase.ResultKindResult,
			Service: "attendance", OperationID: "listAttendances",
		},
	}

	want := "質問: itm-001の詳細\n" +
		"直前: 在庫管理 / listInventoryItems（在庫の一覧を見せて）\n" +
		"直前: attendance / listAttendances（勤怠の方も見せて）\n" +
		"\n候補:\n" +
		"listInventoryItems\t在庫管理\t在庫の一覧を返す\n" +
		lineListCapabilities + "\n" + lineProposePanel + "\n" + lineNone

	assert.Equal(t, want, userMessage("itm-001の詳細", nil, turns, catalog))
}
