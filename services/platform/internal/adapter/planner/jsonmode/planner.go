// Package jsonmode implements usecase.Planner for models that cannot call
// tools at all: it renders usecase.ToolsFor's catalogue as text in the
// system prompt and asks for a single JSON object back, over the same
// internal/adapter/planner/chat transport internal/adapter/planner/toolcall
// uses. It is the other half of the Planner port (D5,
// docs/specs/orchestration.md); see toolcall's own package doc for the tool
// calling half.
package jsonmode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// The five shapes a JSON answer's "kind" can name - the JSON planner's
// equivalent of a tool call's name, mirroring usecase.DecisionKind exactly
// (docs/plans/orchestration.md, Task 11; docs/plans/proposing.md, Task 0).
const (
	kindCall             = "call"
	kindAsk              = "ask"
	kindNone             = "none"
	kindListCapabilities = "list_capabilities"
	kindProposePanel     = "propose_panel"
)

// maxAttempts bounds how many times Plan asks the model for a decision: one
// initial attempt, plus the one retry Task 11 Step 3/4 asks for. A second
// bad answer in a row gives up rather than retrying forever.
const maxAttempts = 2

// ErrEmptyResponse is returned when the model's message carries no content
// to parse at all.
var ErrEmptyResponse = errors.New("planner returned an empty response")

// ErrInvalidJSON is returned when the model's content is not parseable as
// the expected JSON object, after stripping any surrounding prose or
// markdown fence.
var ErrInvalidJSON = errors.New("planner returned invalid JSON")

// roleUser mirrors chat.Message.Role's "user" value: named once so the
// literal doesn't drift and to satisfy goconst
// (harness/quality/go/golangci.yml, min-occurrences: 3).
const roleUser = "user"

// ErrTruncated stands in for a parse error when the model's answer was cut
// off by chat.MaxTokens (defect 3, docs/specs/shortlisting.md - see
// chat.MaxTokens's own doc comment for the measured reproduction) before it
// could finish: there is nothing to parse, valid or not, so this is fed
// into the same retry path as ErrInvalidJSON rather than treated as a
// distinct failure - and, unlike a genuinely invalid answer, exhausting the
// retries on this alone ends in usecase.DecisionNone, not an error, since
// there is no bug in the model's JSON to report - it simply ran out of
// budget.
var ErrTruncated = errors.New("planner's answer was truncated by max_tokens")

// ErrUnknownKind is returned when a parsed JSON object's "kind" is none of
// call, ask, none or list_capabilities.
var ErrUnknownKind = errors.New("planner returned an unknown kind")

// ErrUnknownOperation is returned when a "call" answer names a service and
// operation id that is not in the catalogue this Planner was built with.
// Unlike toolcall.ErrUnknownOperation, this can only be a hallucination,
// never an ambiguity: the JSON answer names its own service, so there is no
// resolveService-style guess to make (see Planner's own doc comment).
var ErrUnknownOperation = errors.New("planner named an operation not in the catalogue")

// ErrEnumValue is returned when a "call" or "ask" answer's args name a
// value outside a parameter's declared enum - the check that stands in for
// tool calling's strict: true, which this transport has no equivalent of
// (docs/specs/orchestration.md, section 8).
var ErrEnumValue = errors.New("planner returned a value outside the parameter's enum")

// systemPromptHeaderPrefix is prepended to the rendered catalogue: the
// call/ask/list_capabilities shapes every request can use, regardless of
// whether propose_panel is offered. It is English even though every
// example and every question in it is Japanese, the same convention
// internal/adapter/planner/toolcall's systemPrompt follows.
const systemPromptHeaderPrefix = "You are given a catalogue of internal services below, rendered as plain text " +
	"because this model cannot call tools. Read the user's question, in Japanese, and reply with exactly one " +
	"JSON object and nothing else - no markdown fence, no explanation before or after it. Its shape is one of:\n\n" +
	`{"kind":"call","service":<service>,"operationId":<operationId>,"args":{...}}` + "  call one operation that " +
	"answers the question, with only the parameters it declares.\n" +
	`{"kind":"ask","service":<service>,"operationId":<operationId>,"param":<name>,"question":<question, in Japanese>}` +
	"  when a parameter's enum value cannot be told from the question - the service and operationId are the " +
	"operation you would have called instead, had the value been clear.\n" +
	`{"kind":"list_capabilities","service":<service, optional>}` + "  when the question asks what can be done at " +
	"all, rather than asking to do something.\n"

// systemPromptProposePanelBlock describes the "propose_panel" shape - only
// ever appended to the prompt when the request carries a workspace id
// (O2/O3, docs/specs/offering.md): a question with none has no workspace
// to put a panel on, and telling the model about a shape it should never
// use is exactly the prompt-the-model-did-not-need section 5's third
// exclusion warns against.
const systemPromptProposePanelBlock = `{"kind":"propose_panel","service":<service>,"operationId":<operationId>,"args":{...},` +
	`"component":<component, optional>,"chart":{"category":<field>,"value":<field>,"kind":<bar|line|pie>} (optional),` +
	`"transform":{"groupBy":<field>,"aggregate":<count|sum|avg>,"field":<field, optional>} (optional),` +
	`"title":<title, optional>}` + "  when the question asks to put something on the workspace's screen " +
	"rather than asking to look something up - name the operation and args exactly as \"call\" would, and " +
	"leave component/chart/transform/title out to let the platform fill them in.\n"

// systemPromptHeaderSuffix closes the header: the "none" shape, the enum
// reminder, and the "Catalogue:" label the rendered catalogue follows.
const systemPromptHeaderSuffix = `{"kind":"none"}` + "  when nothing in the catalogue answers the question.\n\n" +
	"Only ever use a value listed in a parameter's own enum; never invent one that is not listed.\n\nCatalogue:\n"

// responseFormatName names the JSON schema sent in ResponseFormat.
const responseFormatName = "orchestra_decision"

// Planner implements usecase.Planner by rendering catalog as text and
// asking a model for a single JSON object over chat.Client, per
// docs/plans/orchestration.md Task 11.
//
// Both the system prompt and the response format schema are precomputed in
// two variants - with and without propose_panel - rather than assembled on
// every Plan call: whether propose_panel is offered depends only on
// usecase.PlanContext (O2), which is one of exactly two states today, so
// there are only ever two prefixes the prompt cache needs to be warm for
// (M3, docs/specs/context.md section 4), not one recomputed per request.
type Planner struct {
	client  *chat.Client
	catalog domain.Catalog

	systemPromptWithProposePanel    string
	systemPromptWithoutProposePanel string

	responseFormatWithProposePanel    *chat.ResponseFormat
	responseFormatWithoutProposePanel *chat.ResponseFormat

	// clock is what buildMessages asks for "today" (WithClock's own doc
	// comment, toolcall.WithClock's equivalent for this planner):
	// time.Now by default, overridable so a test can pin an exact
	// instant (dev-stack defect, 2026-09-16: a create form's date was
	// invented outright because the model was never told what day it
	// is).
	clock func() time.Time
}

var _ usecase.Planner = (*Planner)(nil)

// Option configures a Planner beyond client and catalog. WithClock is the
// only one today.
type Option func(*Planner)

// WithClock overrides the source of "today" buildMessages prefixes every
// user message with (New's default is time.Now). Tests are the only real
// caller - see toolcall.WithClock, this planner's own equivalent.
func WithClock(clock func() time.Time) Option {
	return func(p *Planner) {
		p.clock = clock
	}
}

// New builds a Planner. catalog is rendered into the system prompt once,
// here, rather than on every Plan call, and is kept to validate a "call" or
// "ask" answer's service, operation id and enum values against afterwards -
// the same reason toolcall.New keeps it, though for a different question:
// toolcall.Planner needs catalog to resolve the service an operation id
// belongs to, because a tool call names only the operation. A JSON answer
// names its own service (it is just another field of the object), so there
// is no equivalent ambiguity here - catalog is needed for validation, not
// resolution (see DECISIONS.md).
func New(client *chat.Client, catalog domain.Catalog, opts ...Option) *Planner {
	catalogText := renderCatalog(catalog)

	p := &Planner{
		client:  client,
		catalog: catalog,

		systemPromptWithProposePanel: systemPromptHeaderPrefix + systemPromptProposePanelBlock +
			systemPromptHeaderSuffix + catalogText,
		systemPromptWithoutProposePanel: systemPromptHeaderPrefix + systemPromptHeaderSuffix + catalogText,

		responseFormatWithProposePanel:    buildResponseFormat(true),
		responseFormatWithoutProposePanel: buildResponseFormat(false),

		clock: time.Now,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Plan sends query (with answers folded in) and the rendered catalogue,
// followed by turns, to the model, parses and validates the one JSON
// object it answers with, and retries once - quoting the failure back -
// when that fails.
//
// tools decides, once per call, which of the two system prompt and
// response format variants this request gets (O2, docs/specs/offering.md):
// propose_panel's own shape is described, and its kind allowed by the
// response schema, only when tools carries it - i.e. only when
// usecase.ToolsFor's own condition for it held. A "propose_panel" answer
// that arrives anyway (offerProposePanel false) is refused exactly as an
// unknown operation is (see parse, decisionFromProposePanel) - the same
// discipline toolcall.Planner's caller (Orchestrator.Plan) applies, since
// this transport has no equivalent of "the model was never offered the
// function at all" to lean on alone (O5).
//
// turns is rendered after the chosen base prompt (systemPromptFor), never
// before it or inside it: the base itself is one of the two fixed strings
// precomputed in New, so its own bytes stay the same across every question
// of a conversation (M3).
// thinking is accepted only to satisfy usecase.Planner: this transport has
// no equivalent of chat_template_kwargs's enable_thinking (D5,
// docs/specs/orchestration.md - Task 11 is the JSON-mode half of the
// planner port, for a model that cannot call tools at all, and a per-
// request thinking override was only ever measured against the toolcall
// planner and Qwen3.5, docs/specs/shortlisting.md), so it is ignored here.
func (p *Planner) Plan(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, tools []usecase.Tool, _ *bool,
) (usecase.Decision, error) {
	offerProposePanel := toolOffered(tools, usecase.ProposePanelToolName)

	messages := buildMessages(p.systemPromptFor(turns, offerProposePanel), query, answers, p.clock())

	var lastErr error

	truncated := false

	for range maxAttempts {
		resp, err := p.complete(ctx, messages, offerProposePanel)
		if err != nil {
			return usecase.Decision{}, err
		}

		// Defect 3 (docs/specs/shortlisting.md, measured 2026-09-15;
		// chat.MaxTokens's own doc comment carries the reproduction): a
		// truncated answer is never valid JSON to parse - there is nothing
		// here worth decoding, only worth logging so the next person can
		// see what the repetition loop looked like without reproducing the
		// 120s timeout that found it.
		if resp.FinishReason == chat.FinishReasonLength {
			slog.Default().WarnContext(ctx, "planner truncated by max_tokens",
				slog.String("content", chat.Preview(resp.Message.Content)))

			truncated = true
			lastErr = ErrTruncated
			messages = append(messages,
				chat.Message{Role: "assistant", Content: resp.Message.Content},
				chat.Message{Role: roleUser, Content: retryPrompt(resp.Message.Content, ErrTruncated)},
			)

			continue
		}

		truncated = false

		decision, parseErr := p.parse(resp.Message.Content, offerProposePanel)
		if parseErr == nil {
			return decision, nil
		}

		lastErr = parseErr
		messages = append(messages,
			chat.Message{Role: "assistant", Content: resp.Message.Content},
			chat.Message{Role: roleUser, Content: retryPrompt(resp.Message.Content, parseErr)},
		)
	}

	// Every attempt was truncated: this is "no usable decision", the same
	// as an empty tool call list would be for toolcall.Planner, never a
	// reason to answer with an error (defect 3) - unlike exhausting the
	// retries on a genuinely invalid answer, which still surfaces as an
	// error below.
	if truncated {
		return usecase.Decision{Kind: usecase.DecisionNone}, nil
	}

	return usecase.Decision{}, fmt.Errorf("planner gave up after %d attempts: %w", maxAttempts, lastErr)
}

// toolOffered reports whether name is one of tools - the list
// usecase.ToolsFor built for this one request. See Plan's own doc comment.
func toolOffered(tools []usecase.Tool, name string) bool {
	for i := range tools {
		if tools[i].Name == name {
			return true
		}
	}

	return false
}

// complete sends messages with the responseFormat variant offerProposePanel
// selects, and - only when the endpoint answers that request with a
// non-2xx status - retries once more without it. Not every
// OpenAI-compatible backend accepts response_format
// (docs/plans/orchestration.md, Task 11); this is that fallback, verified
// by hand against llama.cpp/llama-swap, which does accept it (DECISIONS.md).
// A failure of the plain retry is returned as-is: at that point the
// endpoint itself is unreachable or erroring for a reason response_format
// was never the cause of.
func (p *Planner) complete(ctx context.Context, messages []chat.Message, offerProposePanel bool) (chat.Response, error) {
	responseFormat := p.responseFormatWithoutProposePanel
	if offerProposePanel {
		responseFormat = p.responseFormatWithProposePanel
	}

	resp, err := p.client.Complete(ctx, &chat.Request{
		Messages: messages, ResponseFormat: responseFormat, Temperature: chat.Zero(), MaxTokens: chat.MaxTokens(),
	})
	if err == nil {
		return resp, nil
	}

	if !errors.Is(err, chat.ErrRequestFailed) {
		return chat.Response{}, fmt.Errorf("calling chat completion: %w", err)
	}

	resp, err = p.client.Complete(ctx, &chat.Request{
		Messages: messages, Temperature: chat.Zero(), MaxTokens: chat.MaxTokens(),
	})
	if err != nil {
		return chat.Response{}, fmt.Errorf("calling chat completion without response_format: %w", err)
	}

	return resp, nil
}

// wireChart and wireTransform are propose_panel's own "chart" and
// "transform" arguments, decoded separately from wireDecision's own flat
// fields because they are nested objects, not scalars.
type wireChart struct {
	Category string `json:"category"`
	Value    string `json:"value"`
	Kind     string `json:"kind"`
}

type wireTransform struct {
	GroupBy   string `json:"groupBy"`
	Aggregate string `json:"aggregate"`
	Field     string `json:"field"`
}

// wireDecision is the JSON object shape the model is asked for - every
// field of every kind at once, since which ones matter depends on Kind
// (see parse).
type wireDecision struct {
	Kind        string         `json:"kind"`
	Service     string         `json:"service"`
	OperationID string         `json:"operationId"`
	Args        map[string]any `json:"args"`
	Param       string         `json:"param"`
	Question    string         `json:"question"`
	Component   string         `json:"component"`
	Chart       *wireChart     `json:"chart"`
	Transform   *wireTransform `json:"transform"`
	Title       string         `json:"title"`
}

// parse extracts a JSON object from content, decodes it, and maps it onto a
// usecase.Decision - validating a "call" answer's service, operation id and
// argument enums along the way (see decisionFromCall).
//
// offerProposePanel is Plan's own (see its doc comment): a "propose_panel"
// kind arriving when it is false is refused exactly like a kind this
// planner never recognises at all (ErrUnknownKind) - the list is what the
// model was offered, not what the platform trusts (O5,
// docs/specs/offering.md).
func (p *Planner) parse(content string, offerProposePanel bool) (usecase.Decision, error) {
	extracted := extractJSON(content)
	if extracted == "" {
		return usecase.Decision{}, ErrEmptyResponse
	}

	var wire wireDecision
	if err := json.Unmarshal([]byte(extracted), &wire); err != nil {
		return usecase.Decision{}, fmt.Errorf("%w: %w: %s", ErrInvalidJSON, err, extracted)
	}

	switch wire.Kind {
	case kindNone:
		return usecase.Decision{Kind: usecase.DecisionNone}, nil
	case kindListCapabilities:
		return usecase.Decision{Kind: usecase.DecisionListCapabilities, Service: wire.Service}, nil
	case kindAsk:
		return usecase.Decision{
			Kind: usecase.DecisionAsk, Service: wire.Service, OperationID: wire.OperationID,
			Param: wire.Param, Question: wire.Question,
		}, nil
	case kindCall:
		return p.decisionFromCall(&wire)
	case kindProposePanel:
		if !offerProposePanel {
			return usecase.Decision{}, fmt.Errorf("%w: %q", ErrUnknownKind, wire.Kind)
		}

		return p.decisionFromProposePanel(&wire)
	default:
		return usecase.Decision{}, fmt.Errorf("%w: %q", ErrUnknownKind, wire.Kind)
	}
}

// decisionFromCall validates a "call" answer's service/operationId against
// catalog and its args against that endpoint's parameter (and request
// body) schemas, then builds the DecisionCall.
func (p *Planner) decisionFromCall(wire *wireDecision) (usecase.Decision, error) {
	endpoint, ok := p.catalog.Find(wire.Service, wire.OperationID)
	if !ok {
		return usecase.Decision{}, fmt.Errorf("%w: %s.%s", ErrUnknownOperation, wire.Service, wire.OperationID)
	}

	if err := validateArgs(&endpoint, wire.Args); err != nil {
		return usecase.Decision{}, err
	}

	return usecase.Decision{
		Kind: usecase.DecisionCall, Service: wire.Service, OperationID: wire.OperationID, Args: wire.Args,
	}, nil
}

// decisionFromProposePanel validates a "propose_panel" answer's
// service/operationId and args exactly as decisionFromCall does - the same
// reason: this transport's response_format has no equivalent of tool
// calling's strict: true, and a proposal's args are exactly a call's own
// args, just answered with instead of run (docs/specs/proposing.md,
// section 3-4). Component, chart, transform and title are read only when
// the model gave them; Orchestrator.propose fills in whatever it left out
// from the catalogue.
func (p *Planner) decisionFromProposePanel(wire *wireDecision) (usecase.Decision, error) {
	endpoint, ok := p.catalog.Find(wire.Service, wire.OperationID)
	if !ok {
		return usecase.Decision{}, fmt.Errorf("%w: %s.%s", ErrUnknownOperation, wire.Service, wire.OperationID)
	}

	if err := validateArgs(&endpoint, wire.Args); err != nil {
		return usecase.Decision{}, err
	}

	return usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     wire.Service,
		OperationID: wire.OperationID,
		Args:        wire.Args,
		Component:   domain.Component(wire.Component),
		View:        viewFromWire(wire),
		Title:       wire.Title,
	}, nil
}

// viewFromWire builds propose_panel's optional view from wire's "chart"
// and "transform" objects, mirroring domain.View's own two independent
// halves (docs/specs/dashboard.md, P1): nil when the model gave neither,
// so Orchestrator.propose's catalogue fallback sees no model-supplied view
// at all rather than an empty one.
func viewFromWire(wire *wireDecision) *domain.View {
	var chart *domain.Chart

	var transform *domain.Transform

	if wire.Chart != nil {
		chart = &domain.Chart{Category: wire.Chart.Category, Value: wire.Chart.Value, Kind: domain.ChartKind(wire.Chart.Kind)}
	}

	if wire.Transform != nil {
		transform = &domain.Transform{
			GroupBy: wire.Transform.GroupBy, Aggregate: domain.Aggregate(wire.Transform.Aggregate), Field: wire.Transform.Field,
		}
	}

	if chart == nil && transform == nil {
		return nil
	}

	return &domain.View{Chart: chart, Transform: transform}
}

// validateArgs rejects any argument naming a value outside its parameter's
// declared enum - the check that stands in for strict: true (see
// ErrEnumValue). A parameter args does not mention, or one with no enum at
// all, is not this function's concern: an endpoint's own required-parameter
// checking happens downstream, at /api/invoke (docs/specs/orchestration.md,
// section 5).
func validateArgs(e *domain.Endpoint, args map[string]any) error {
	for name, schema := range argSchemas(e) {
		if len(schema.Enum) == 0 {
			continue
		}

		raw, ok := args[name]
		if !ok {
			continue
		}

		value, ok := raw.(string)
		if !ok || slices.Contains(schema.Enum, value) {
			continue
		}

		return fmt.Errorf("%w: %s=%q (allowed: %s)", ErrEnumValue, name, value, strings.Join(schema.Enum, ", "))
	}

	return nil
}

// argSchemas collects the schema for every argument name an endpoint
// accepts: its own parameters, plus an object request body's properties
// merged in at the top level - the same shape usecase.inputSchemaFor builds
// for a tool's InputSchema (internal/usecase/tools.go), read here from
// domain.Endpoint directly instead, since that is what validateArgs and
// renderCatalog both already have in hand.
func argSchemas(e *domain.Endpoint) map[string]domain.Schema {
	out := make(map[string]domain.Schema, len(e.Parameters))

	for i := range e.Parameters {
		p := &e.Parameters[i]
		out[p.Name] = p.Schema
	}

	if e.RequestBody != nil && e.RequestBody.Type == domain.SchemaTypeObject {
		maps.Copy(out, e.RequestBody.Properties)
	}

	return out
}

// extractJSON trims content down to the JSON object it names: a model
// asked to answer with only JSON sometimes wraps it in a markdown fence or
// a sentence anyway, so this strips a leading/trailing ``` fence and then
// takes the substring between the first "{" and the last "}". An empty
// result means no object was found at all.
func extractJSON(content string) string {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')

	if start == -1 || end == -1 || end < start {
		return ""
	}

	return trimmed[start : end+1]
}

// retryPrompt builds the user turn sent after a bad answer: it quotes both
// the rejected content and the reason it was rejected (ErrEnumValue's
// message names the offending value and the parameter's allowed list), per
// docs/plans/orchestration.md Task 11 Step 3.
func retryPrompt(previous string, err error) string {
	return fmt.Sprintf(
		"前回の回答は無効でした: %s\n前回の回答: %s\n"+
			"指定した JSON の形式で、正しい値のみを使って、もう一度だけ答え直してください。",
		err, previous,
	)
}

// turnsIntro is prepended to the rendered conversation history (see
// renderTurns). Mirrors internal/adapter/planner/toolcall's own constant
// of the same name and wording - the two planners tell the model the same
// thing about the same Turn slice, just at a different point in a
// differently-shaped prompt.
const turnsIntro = "Here is the conversation so far, oldest first. Each line is a question the user " +
	"already asked and what the platform decided to do about it - service, operation and arguments, " +
	"never the data the operation returned. Use it only to understand what \"it\", \"the same thing\" " +
	"or an unnamed service in the new question below refers to."

// systemPromptFor picks the base system prompt offerProposePanel selects -
// one of the two fixed strings New precomputed - and returns it unchanged
// when there are no turns, or with renderTurns appended after it otherwise.
// The base itself is never mutated or recomputed here - only ever
// concatenated with something after it - so its own bytes are identical on
// every call for the same offerProposePanel, regardless of turns (M3).
func (p *Planner) systemPromptFor(turns []usecase.Turn, offerProposePanel bool) string {
	base := p.systemPromptWithoutProposePanel
	if offerProposePanel {
		base = p.systemPromptWithProposePanel
	}

	if len(turns) == 0 {
		return base
	}

	return base + "\n\n" + renderTurns(turns)
}

// renderTurns renders turns as turnsIntro followed by one line each,
// naming the question, the ResultKind it resolved to, and - when a
// service was decided (absent for usecase.ResultKindNone) - the service,
// operation id and arguments the platform called it with. usecase.Turn has
// no field for the answer's own data (M1), so there is nothing here that
// could render a row of one even by mistake.
func renderTurns(turns []usecase.Turn) string {
	var b strings.Builder

	b.WriteString(turnsIntro)

	for _, t := range turns {
		fmt.Fprintf(&b, "\n- question: %q, kind=%s", t.Question, t.Kind)

		if t.Service == "" {
			continue
		}

		fmt.Fprintf(&b, ", service=%s, operationId=%s", t.Service, t.OperationID)

		if len(t.Args) == 0 {
			continue
		}

		argsJSON, err := json.Marshal(t.Args)
		if err != nil {
			argsJSON = []byte("{}")
		}

		fmt.Fprintf(&b, ", args=%s", argsJSON)
	}

	return b.String()
}

// buildMessages renders one system message (systemPrompt, already carrying
// the rendered catalogue and, when there are any, the conversation so far
// after it) and one user message: dateLine first, on every call, then the
// question, followed - when answers is non-empty - by every answer
// already given to a previous ask, exactly as
// internal/adapter/planner/toolcall.buildMessages does, for the same
// reason (D8: one stateless chat completion per request).
func buildMessages(systemPrompt, query string, answers []usecase.Answer, now time.Time) []chat.Message {
	var b strings.Builder

	b.WriteString(dateLine(now))
	b.WriteString(query)

	if len(answers) > 0 {
		b.WriteString("\n\nこれまでに確認した値:\n")

		for _, a := range answers {
			fmt.Fprintf(&b, "- %s = %s\n", a.Param, a.Value)
		}

		b.WriteString("\n上記の値をそのまま使って、対応する操作を呼び出してください。")
	}

	return []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: roleUser, Content: b.String()},
	}
}

// dateLine is buildMessages's own first line of the user message, on every
// call - see toolcall.dateLine, this planner's own equivalent (duplicated
// rather than shared: each planner adapter already renders its own turns
// and answers independently, for the same reason).
func dateLine(now time.Time) string {
	return fmt.Sprintf("今日は %s（%s）です。\n\n", now.Format("2006-01-02"), japaneseWeekday(now.Weekday()))
}

// japaneseWeekday renders a time.Weekday as its single full-width Japanese
// character - see toolcall.japaneseWeekday, this planner's own duplicate.
func japaneseWeekday(d time.Weekday) string {
	return [...]string{"日", "月", "火", "水", "木", "金", "土"}[d]
}

// renderCatalog renders catalog as the text block described in
// docs/plans/orchestration.md Task 11: one line per endpoint with its
// service, operation id and summary, followed by one line per parameter
// (and, for an unsafe operation, per request body property) naming its
// type and - when it is an enum - every allowed value with its Japanese
// label. This is the same catalogue usecase.ToolsFor shapes into tool
// definitions for internal/adapter/planner/toolcall, rendered here as plain
// text instead because this planner's model cannot call tools at all.
func renderCatalog(catalog domain.Catalog) string {
	var b strings.Builder

	for i := range catalog.Endpoints {
		e := &catalog.Endpoints[i]

		fmt.Fprintf(&b, "- service=%s operationId=%s: %s\n", e.Service, e.OperationID, e.Summary)

		schemas := argSchemas(e)
		for _, name := range sortedNames(schemas) {
			schema := schemas[name]
			renderParam(&b, name, &schema)
		}
	}

	return b.String()
}

// renderParam writes one parameter's line: its name, JSON Schema type, when
// it declares one, its enum values with their Japanese labels, and - when
// the schema's contract declares one - its own Description text. Without
// this, a parameter's Description would reach the tool-calling planner's
// property description but never this planner's prompt at all, since
// nothing else here reads schema.Description.
func renderParam(b *strings.Builder, name string, schema *domain.Schema) {
	fmt.Fprintf(b, "  param %s (%s", name, schema.Type)

	if len(schema.Enum) > 0 {
		b.WriteString(", enum: " + enumWithLabels(schema.Enum, schema.EnumLabels))
	}

	b.WriteString(")")

	if schema.Description != "" {
		b.WriteString(": " + schema.Description)
	}

	b.WriteString("\n")
}

// enumWithLabels renders an enum's values and Japanese labels as
// "allocated=引当済 / quarantined=検品保留", the same shape
// usecase.enumLabels renders for a tool-calling schema's description
// (internal/usecase/tools.go) - reimplemented here rather than exported
// from usecase, since usecase must not depend on this adapter and this
// adapter must not depend on usecase's unexported helpers either.
func enumWithLabels(enum []string, labels map[string]string) string {
	parts := make([]string, len(enum))
	for i, value := range enum {
		parts[i] = value + "=" + labels[value]
	}

	return strings.Join(parts, " / ")
}

// sortedNames returns m's keys in sorted order, so renderCatalog's output -
// and therefore the prompt itself - is deterministic regardless of Go's
// randomized map iteration.
func sortedNames(m map[string]domain.Schema) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// decisionSchemaJSON is the JSON Schema sent as ResponseFormat: a permissive
// object naming every field any kind of answer might use, with only "kind"
// required. It is deliberately not a stricter oneOf keyed on kind -
// docs/specs/orchestration.md notes strict mode has no equivalent for this
// transport, so the real validation is parse/validateArgs in Go, not the
// grammar this schema constrains the model's tokens with.
//
// It is a hand-written json.RawMessage, not a map[string]any built the way
// every other schema in this codebase is: encoding/json always marshals a
// map's keys in sorted order, which would put "args" before "kind" and
// "service" - and a grammar-constrained local model (llama.cpp/llama-swap,
// qwen3.5-9b-q8) was observed, by hand, to corrupt its own JSON under that
// reordering, splicing broken escape sequences into "service"'s value where
// the schema forced it earlier than the model's own generation order
// wanted it. The same catalogue and question produced clean JSON once the
// schema's property order was made to match the order every kind's example
// already appears in systemPromptHeader (kind, service, operationId,
// args, param, question). See DECISIONS.md.
// decisionSchemaKinds is decisionSchemaJSON's "kind" enum, with
// propose_panel appended only when offerProposePanel (O2,
// docs/specs/offering.md) - a grammar-constrained model literally cannot
// emit a "kind" this omits, which is this transport's own equivalent of a
// tool never being declared to a tool-calling model at all.
func decisionSchemaKinds(offerProposePanel bool) string {
	kinds := `"` + kindCall + `","` + kindAsk + `","` + kindNone + `","` + kindListCapabilities + `"`
	if offerProposePanel {
		kinds += `,"` + kindProposePanel + `"`
	}

	return kinds
}

// decisionSchemaJSON builds the JSON Schema sent as ResponseFormat: a
// permissive object naming every field any kind of answer might use, with
// only "kind" required. It is deliberately not a stricter oneOf keyed on
// kind - docs/specs/orchestration.md notes strict mode has no equivalent
// for this transport, so the real validation is parse/validateArgs in Go,
// not the grammar this schema constrains the model's tokens with.
//
// It is hand-written, not a map[string]any built the way every other
// schema in this codebase is: encoding/json always marshals a map's keys
// in sorted order, which would put "args" before "kind" and "service" -
// and a grammar-constrained local model (llama.cpp/llama-swap,
// qwen3.5-9b-q8) was observed, by hand, to corrupt its own JSON under that
// reordering, splicing broken escape sequences into "service"'s value where
// the schema forced it earlier than the model's own generation order
// wanted it. The same catalogue and question produced clean JSON once the
// schema's property order was made to match the order every kind's example
// already appears in systemPromptHeaderPrefix (kind, service, operationId,
// args, param, question). See DECISIONS.md.
func decisionSchemaJSON(offerProposePanel bool) string {
	return `{"type":"object","properties":{` +
		`"kind":{"type":"string","enum":[` + decisionSchemaKinds(offerProposePanel) + `]},` +
		`"service":{"type":"string"},` +
		`"operationId":{"type":"string"},` +
		`"args":{"type":"object"},` +
		`"param":{"type":"string"},` +
		`"question":{"type":"string"},` +
		`"component":{"type":"string"},` +
		`"chart":{"type":"object","properties":{"category":{"type":"string"},"value":{"type":"string"},"kind":{"type":"string"}}},` +
		`"transform":{"type":"object","properties":{"groupBy":{"type":"string"},"aggregate":{"type":"string"},"field":{"type":"string"}}},` +
		`"title":{"type":"string"}` +
		`},"required":["kind"]}`
}

// buildResponseFormat builds the ResponseFormat sent with a request whose
// offerProposePanel this is - see decisionSchemaJSON for what it
// constrains and why it is not a map[string]any like every other schema
// this package builds.
func buildResponseFormat(offerProposePanel bool) *chat.ResponseFormat {
	return &chat.ResponseFormat{
		Type: "json_schema",
		JSONSchema: map[string]any{
			"name":   responseFormatName,
			"schema": json.RawMessage(decisionSchemaJSON(offerProposePanel)),
		},
	}
}
