package pick

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// pickMaxTokens mirrors e2e/narrowing/pick/client.ts's own max_tokens: the
// pick's whole answer is one short line, never the fill's own argument
// budget (AC-S-104, docs/specs/staging.md section 4).
const pickMaxTokens = 200

// Picker implements usecase.Picker over chat.Client, in the measured
// stand-in picker's own request shape: temperature 0, thinking off,
// regardless of the request's own thinking (S5) - the fill
// (internal/usecase/orchestrator_preferred.go) is where that switch
// applies.
type Picker struct {
	client chat.Completer
	model  string
}

var _ usecase.Picker = (*Picker)(nil)

// New builds a Picker over client, sending model on every request - the
// same model the platform's toolcall.Planner is configured with (S6: "the
// pick's model is the planner's model"). client is a chat.Completer, not a
// concrete *chat.Client, so the same Picker works unchanged over either
// chat backend (chat.Client's OpenAI-compatible transport, or
// internal/adapter/planner/chat/anthropic.Client).
func New(client chat.Completer, model string) *Picker {
	return &Picker{client: client, model: model}
}

// Pick sends query, answers, turns and shortlist to the model in the
// pick's own format (userMessage) and parses the one line it answers with
// back into a usecase.Pick. An empty shortlist is PickNone without calling
// the model at all - there is nothing to pick from.
//
// turns is rendered into the user message (turnLines) since 2026-09-17
// (docs/measurements/jev-v5.md's isolation result: giving the pick stage
// the turns recovered follow-up-other-service from 0/10 to 10/10 for the
// Jev picker - the local pick had the same blindness). SystemPrompt still
// stays byte-identical to e2e/narrowing/pick/client.ts's own
// PICK_SYSTEM_PROMPT (S2's cross-language comparison, prompt_test.go): only
// the user message changes, and only when turns is non-empty -
// TestPickByteIdenticalWithNoTurns (picker_test.go) is the explicit
// assertion that a request built with no turns is still byte-identical to
// one built before turns existed.
func (p *Picker) Pick(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
	planCtx usecase.PlanContext,
) (usecase.Pick, error) {
	if len(shortlist.Endpoints) == 0 {
		return usecase.Pick{Kind: usecase.PickNone}, nil
	}

	offerProposePanel := planCtx.WorkspaceID != ""

	maxTokens := pickMaxTokens

	resp, err := p.client.Complete(ctx, &chat.Request{
		Model: p.model,
		Messages: []chat.Message{
			{Role: "system", Content: SystemPrompt},
			{Role: "user", Content: userMessage(query, answers, turns, shortlist, offerProposePanel)},
		},
		Temperature:        chat.Zero(),
		MaxTokens:          &maxTokens,
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
	})
	if err != nil {
		return usecase.Pick{}, fmt.Errorf("picking: %w", err)
	}

	// A response cut off at max_tokens is treated the same as no answer
	// at all, the same reasoning toolcall.Planner's own truncation check
	// gives (chat.MaxTokens's own doc comment): whatever partial line the
	// model managed to emit is not a candidate id to trust.
	if resp.FinishReason == chat.FinishReasonLength {
		slog.Default().WarnContext(ctx, "pick truncated by max_tokens",
			slog.String("content", chat.Preview(resp.Message.Content)))

		return usecase.Pick{Kind: usecase.PickNone}, nil
	}

	return parse(resp.Message.Content, candidatesFor(shortlist, offerProposePanel)), nil
}
