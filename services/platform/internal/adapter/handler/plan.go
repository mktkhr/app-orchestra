package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// errUnrenderableData marks a usecase.Result whose Data is not a JSON
// object: the /api/plan contract's "data" field is one (PlanResult.Data is
// map[string]interface{}), because every endpoint Render draws a
// table or detail component for produces one - a bare array or scalar
// response renders as no component at all (see domain.Render) and never
// reaches here.
var errUnrenderableData = errors.New("result data is not a JSON object")

// planner is what Plan needs from the orchestration layer: satisfied by
// *usecase.Orchestrator. An interface here, rather than the concrete type,
// keeps this handler's test doubles simple.
type planner interface {
	Plan(
		ctx context.Context, user *domain.User, query string, answers []usecase.Answer, turns []usecase.Turn,
	) (usecase.Result, error)
}

// Plan implements the "plan" tag of the generated strict server interface:
// POST /api/plan.
type Plan struct {
	orchestrator planner
}

// NewPlan builds the /api/plan handler over orchestrator.
func NewPlan(orchestrator planner) *Plan {
	return &Plan{orchestrator: orchestrator}
}

// PostPlan turns a question into a decision and, when it was safe to act
// on, its result.
func (h *Plan) PostPlan(
	ctx context.Context,
	request openapi.PostPlanRequestObject,
) (openapi.PostPlanResponseObject, error) {
	result, err := h.orchestrator.Plan(
		ctx, currentUser(ctx), request.Body.Query, toAnswers(request.Body.Answers), toTurns(request.Body.Turns),
	)
	if err != nil {
		return planErrorResponse(err), nil
	}

	apiResult, err := toAPIPlanResult(&result)
	if err != nil {
		return dataErrorResponse(err), nil
	}

	return openapi.PostPlan200JSONResponse(apiResult), nil
}

// planErrorResponse maps an Orchestrator.Plan error onto an HTTP status: a
// DecisionKind this deployment does not implement at all is reported as
// 501; everything else - which no longer includes a disambiguation naming
// a parameter with no enum, since Orchestrator.ask degrades that case to a
// form instead of an error - as 500.
func planErrorResponse(err error) openapi.PostPlanResponseObject {
	if errors.Is(err, usecase.ErrNotImplemented) {
		return openapi.PostPlan501JSONResponse{Message: err.Error()}
	}

	return openapi.PostPlan500JSONResponse{Message: err.Error()}
}

// dataErrorResponse reports a Result that could not be rendered onto the
// wire (see errUnrenderableData) as a 500: this is a bug in the running
// deployment's endpoints, not something the caller did wrong.
func dataErrorResponse(err error) openapi.PostPlanResponseObject {
	return openapi.PostPlan500JSONResponse{Message: err.Error()}
}

// toAnswers converts the wire representation of PlanRequest.Answers into
// usecase.Answer.
func toAnswers(answers *[]openapi.Answer) []usecase.Answer {
	if answers == nil {
		return nil
	}

	out := make([]usecase.Answer, 0, len(*answers))
	for _, a := range *answers {
		out = append(out, usecase.Answer{Param: a.Param, Value: a.Value})
	}

	return out
}

// toTurns converts the wire representation of PlanRequest.Turns into
// usecase.Turn, oldest first - the same order the wire carries them in
// (docs/specs/context.md, section 5). Truncating to the configured window
// is Orchestrator.Plan's job (docs/specs/context.md, section 6), not this
// handler's: it forwards whatever the browser sent, exactly as toAnswers
// forwards every answer without judging how many there are.
func toTurns(turns *[]openapi.Turn) []usecase.Turn {
	if turns == nil {
		return nil
	}

	out := make([]usecase.Turn, 0, len(*turns))
	for _, t := range *turns {
		turn := usecase.Turn{Question: t.Question, Kind: usecase.ResultKind(t.Kind)}

		if t.Service != nil {
			turn.Service = *t.Service
		}

		if t.OperationId != nil {
			turn.OperationID = *t.OperationId
		}

		if t.Args != nil {
			turn.Args = *t.Args
		}

		out = append(out, turn)
	}

	return out
}

// toAPIPlanResult converts a usecase.Result into the wire PlanResult.
//
// result is a pointer, not the value Orchestrator.Plan returns, because
// usecase.Result is over 100 bytes: golangci-lint's gocritic hugeParam
// check (part of the fixed harness policy, see
// harness/quality/go/golangci.yml) rejects passing it by value.
func toAPIPlanResult(result *usecase.Result) (openapi.PlanResult, error) {
	out := openapi.PlanResult{Kind: openapi.DecisionKind(result.Kind)}

	if result.Component != "" {
		component := openapi.Component(result.Component)
		out.Component = &component
	}

	if result.Message != "" {
		out.Message = &result.Message
	}

	if result.Kind == usecase.ResultKindResult {
		data, err := toAPIData(result.Data)
		if err != nil {
			return openapi.PlanResult{}, err
		}

		out.Data = &data
		out.Source = &openapi.Source{
			Service:            result.Service,
			ServiceDisplayName: result.ServiceDisplayName,
			OperationId:        result.OperationID,
		}

		if len(result.Args) > 0 {
			args := result.Args
			out.Source.Args = &args
		}

		if len(result.Fields) > 0 {
			fields := result.Fields
			out.Fields = &fields
		}

		out.View = toAPIView(result.View)
	}

	if result.Kind == usecase.ResultKindForm {
		schema := result.Schema
		out.Schema = &schema
		out.Target = &openapi.Source{
			Service:            result.Service,
			ServiceDisplayName: result.ServiceDisplayName,
			OperationId:        result.OperationID,
		}

		if len(result.Initial) > 0 {
			initial := result.Initial
			out.Initial = &initial
		}
	}

	if result.Kind == usecase.ResultKindAsk {
		out.Question = &result.Question
		out.Param = &result.Param
		out.Options = toAPIOptions(result.Options)
	}

	if result.Kind == usecase.ResultKindProposal {
		panel := toAPIProposedPanel(result)
		out.Panel = &panel
	}

	return out, nil
}

// toAPIProposedPanel converts a usecase.Result carrying a ResultKindProposal
// into the wire ProposedPanel: the same fields a saved Panel carries, minus
// where it sits (docs/specs/proposing.md, section 4) - args defaults to an
// empty object rather than nil, since ProposedPanel.Args is not a pointer
// (the model may propose a panel with no arguments at all, and that is not
// the same as the field being absent from the wire).
//
// result is a pointer, not the value toAPIPlanResult holds, for the same
// gocritic hugeParam reason as toAPIPlanResult's own parameter.
func toAPIProposedPanel(result *usecase.Result) openapi.ProposedPanel {
	args := result.Args
	if args == nil {
		args = map[string]any{}
	}

	return openapi.ProposedPanel{
		Service:     result.Service,
		OperationId: result.OperationID,
		Args:        args,
		Component:   openapi.Component(result.Component),
		Title:       result.Title,
		View:        toAPIView(result.View),
	}
}

// toAPIOptions converts a usecase.Result's catalogue-sourced options into
// the wire Option slice.
func toAPIOptions(options []domain.Option) *[]openapi.Option {
	out := make([]openapi.Option, len(options))
	for i, o := range options {
		out[i] = openapi.Option{Value: o.Value, Label: o.Label}
	}

	return &out
}

// toAPIData asserts a usecase.Result's Data as the JSON object the
// contract requires. See errUnrenderableData.
func toAPIData(data any) (map[string]any, error) {
	if data == nil {
		return map[string]any{}, nil
	}

	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", errUnrenderableData, data)
	}

	return m, nil
}
