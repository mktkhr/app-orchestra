// Package toolcall implements usecase.Planner by asking a model to call
// one of usecase.ToolsFor's tools, over internal/adapter/planner/chat's
// OpenAI-compatible transport. It is the planner for models that support
// tool calling (D5, docs/specs/orchestration.md); internal/adapter/planner/jsonmode
// (Task 11) is the other half of the same port, for models that do not.
package toolcall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/wording"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// toolNameAskUser mirrors usecase.AskUserTool's Name.
const toolNameAskUser = "ask_user"

// toolNameListCapabilities mirrors usecase.ListCapabilitiesTool's Name.
const toolNameListCapabilities = "list_capabilities"

// toolNameProposePanel mirrors usecase.ProposePanelTool's Name.
const toolNameProposePanel = "propose_panel"

// ErrUnknownOperation is returned when the model calls a tool whose name
// is not any endpoint in the catalogue this Planner was built with - a
// hallucinated operation id, which resolveService cannot resolve to any
// service at all.
var ErrUnknownOperation = errors.New("planner named an operation not in the catalogue")

// Planner implements usecase.Planner: it sends usecase.ToolsFor's tools to
// a model over a chat.Completer (chat.Client's OpenAI-compatible
// transport, or internal/adapter/planner/chat/anthropic.Client) and maps
// the tool call it makes onto a usecase.Decision.
type Planner struct {
	client  chat.Completer
	catalog domain.Catalog
	// wording is the named set of words (docs/specs/wording.md) this
	// Planner builds its system message and tool descriptions from -
	// wording.Default() (v1: today's text, byte for byte, AC-Q-101)
	// unless WithWording says otherwise. A pointer, not the 88-byte value:
	// golangci-lint's gocritic hugeParam check (part of the fixed harness
	// policy, see harness/quality/go/golangci.yml) rejects copying it,
	// here and on every function below that reads it.
	wording *wording.Wording
	// thinking is whether Qwen3.5's thinking is left on (the zero value,
	// true - today's behaviour, nothing sent) or turned off via
	// chat.Request.ChatTemplateKwargs (WithThinking(false); DECISIONS.md,
	// probed 2026-09-16).
	thinking bool
	// repeatPenalty and repeatLastN are chat.Request.RepeatPenalty/
	// RepeatLastN, set together by WithRepeatPenalty. repeatPenalty nil
	// means neither is sent - today's behaviour.
	repeatPenalty *float64
	repeatLastN   *int
	// maxTokens is chat.Request.MaxTokens on every planning call - New's
	// default is chat.MaxTokens's own value (1024, today's behaviour),
	// overridable via WithMaxTokens.
	maxTokens int
	// clock is what buildUserContent asks for "today" (WithClock's own
	// doc comment): time.Now by default, so the running platform always
	// prefixes the user message with the real date, but a test can pin
	// it to an exact instant instead (dev-stack defect, 2026-09-16:
	// without any notion of today, the model filled a create form's date
	// with an invented year, "2023-10-10", when asked to record 4月20日).
	clock func() time.Time
}

var _ usecase.Planner = (*Planner)(nil)

// Option configures a Planner beyond client and catalog. WithWording is
// the only one today.
type Option func(*Planner)

// WithClock overrides the source of "today" buildUserContent prefixes
// every user message with (New's default is time.Now, so the platform
// itself always sees the real date). Tests are the only real caller: a
// pinned clock lets a test assert the exact date line without depending
// on when it happens to run.
func WithClock(clock func() time.Time) Option {
	return func(p *Planner) {
		p.clock = clock
	}
}

// WithWording selects the words this Planner uses for its system message,
// its three built-in tools' descriptions and each catalogue tool's own
// description. Omitted, New uses wording.Default() - so every existing
// call site (pkg/app, and every test in this package that predates this
// option) keeps behaving exactly as it did before this option existed
// (AC-Q-101). w is a pointer for the same hugeParam reason Planner.wording
// is.
func WithWording(w *wording.Wording) Option {
	return func(p *Planner) {
		p.wording = w
	}
}

// WithThinking selects whether Qwen3.5's thinking is left on (enabled,
// New's default - today's behaviour, nothing sent) or turned off (Plan
// sends chat_template_kwargs: {"enable_thinking": false} on every
// planning request). Probed 2026-09-16 against llama-server build 10920:
// the model answers with reasoning_content and empty content, at
// max_tokens 1024, when thinking is left on and unaddressed.
func WithThinking(enabled bool) Option {
	return func(p *Planner) {
		p.thinking = enabled
	}
}

// WithRepeatPenalty sets chat.Request.RepeatPenalty and RepeatLastN on
// every planning request (probed 2026-09-16: llama-server honours both on
// /v1/chat/completions, not just /completion). Omitted, New sends
// neither - today's behaviour.
func WithRepeatPenalty(penalty float64, lastN int) Option {
	return func(p *Planner) {
		p.repeatPenalty = &penalty
		p.repeatLastN = &lastN
	}
}

// WithMaxTokens overrides chat.Request.MaxTokens on every planning call
// (New's default is chat.MaxTokens's own value, 1024 - today's behaviour).
// A thinking model needs a bigger budget than a model that does not
// (measured 2026-09-19, docs/specs/shortlisting.md: 34 of 176 calls
// stopped at the fixed 1024 with thinking on, and the corpus score fell
// 81 -> 56).
func WithMaxTokens(maxTokens int) Option {
	return func(p *Planner) {
		p.maxTokens = maxTokens
	}
}

// New builds a Planner. catalog is needed to resolve the service an
// operation id belongs to (see resolveService) - a tool call names only
// the operation, never the service, so the tool-calling wire format alone
// cannot answer that question. Thinking defaults to enabled (today's
// behaviour) unless WithThinking(false) is given.
func New(client chat.Completer, catalog domain.Catalog, opts ...Option) *Planner {
	defaultWording := wording.Default()
	p := &Planner{
		client: client, catalog: catalog, wording: &defaultWording, thinking: true, clock: time.Now,
		maxTokens: *chat.MaxTokens(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Plan sends query (with answers folded in, see buildMessages), turns and
// tools to the model, and maps the one tool call it returns - if any -
// onto a Decision. turns is rendered into the user message, ahead of the
// current question (see buildMessages) - never into tools itself, which
// shapeTools builds the same way regardless of turns (AC-M-102), nor into
// the system message: tools is what the catalogue prompt cache is warm
// for (M3, docs/specs/context.md section 4), and this way neither it nor
// the system message's own bytes change when a conversation grows.
//
// thinking, when non-nil, overrides p.thinking (this Planner's own
// configured default, WithThinking) for this one call: the effective
// value - thinking's value if given, p.thinking otherwise - is what
// decides chatTemplateKwargs and what every log line below reports, never
// the configured default alone (docs/specs/shortlisting.md, "platform
// knobs" subproject, decided 2026-09-16).
func (p *Planner) Plan(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, tools []usecase.Tool,
	thinking *bool,
) (usecase.Decision, error) {
	effectiveThinking := p.thinking
	if thinking != nil {
		effectiveThinking = *thinking
	}

	maxTokens := p.maxTokens

	resp, err := p.client.Complete(ctx, &chat.Request{
		Messages:           buildMessages(query, answers, turns, p.wording.SystemPrompt, p.clock()),
		Tools:              shapeTools(tools, p.wording),
		Temperature:        chat.Zero(),
		MaxTokens:          &maxTokens,
		ChatTemplateKwargs: chatTemplateKwargsFor(effectiveThinking),
		RepeatPenalty:      p.repeatPenalty,
		RepeatLastN:        p.repeatLastN,
	})
	if err != nil {
		return usecase.Decision{}, fmt.Errorf("calling chat completion: %w", err)
	}

	slog.Default().DebugContext(ctx, "planner call completed",
		slog.Int("completion_tokens", resp.Usage.CompletionTokens),
		slog.String("finish_reason", resp.FinishReason),
		slog.Bool("thinking", effectiveThinking))

	// Defect 3 (docs/specs/shortlisting.md, measured 2026-09-15,
	// chat.MaxTokens's own doc comment): a model that hit MaxTokens without
	// finishing (a repetition loop, most often) never produced a usable
	// tool call, whatever partial content or arguments it managed to emit
	// before being cut off - this is "no usable decision", the same as no
	// tool call at all, never a reason to try decoding it. Logged at warn,
	// not error: no request failed, an answer was simply unusable, but the
	// next person debugging a slow or wrong plan needs to see the loop
	// without reproducing the 120s timeout to find it.
	//
	// content alone used to be all this logged - and was empty every time
	// thinking consumed the whole MaxTokens budget (facts, 2026-09-16):
	// reasoning, completion_tokens and thinking are what tell that case
	// (thinking that never finished) apart from a plain repetition loop,
	// without reproducing the run to find out.
	if resp.FinishReason == chat.FinishReasonLength {
		slog.Default().WarnContext(ctx, "planner truncated by max_tokens",
			slog.String("content", chat.Preview(resp.Message.Content)),
			slog.String("reasoning", chat.Preview(resp.Message.ReasoningContent)),
			slog.Int("completion_tokens", resp.Usage.CompletionTokens),
			slog.Bool("thinking", effectiveThinking))

		return usecase.Decision{Kind: usecase.DecisionNone}, nil
	}

	if len(resp.Message.ToolCalls) == 0 {
		return usecase.Decision{Kind: usecase.DecisionNone}, nil
	}

	call := resp.Message.ToolCalls[0].Function

	args, err := decodeArguments(call.Arguments)
	if err != nil {
		return usecase.Decision{}, fmt.Errorf("decoding %s arguments: %w", call.Name, err)
	}

	if call.Name == toolNameAskUser {
		return p.decisionFromAskUser(args), nil
	}

	if call.Name == toolNameListCapabilities {
		return decisionFromListCapabilities(args), nil
	}

	if call.Name == toolNameProposePanel {
		return decisionFromProposePanel(args), nil
	}

	return p.decisionFromCall(call.Name, args)
}

// chatTemplateKwargsFor builds Request.ChatTemplateKwargs for one planning
// call from its effective thinking value (Plan's own doc comment): nil
// (send nothing) when thinking is left on, {"enable_thinking": false}
// otherwise.
func chatTemplateKwargsFor(effectiveThinking bool) map[string]any {
	if effectiveThinking {
		return nil
	}

	return map[string]any{"enable_thinking": false}
}

// decodeArguments parses a tool call's Arguments string as a JSON object.
// An empty string - a tool with no parameters, called with none - decodes
// to an empty map rather than an error.
func decodeArguments(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("parsing arguments %q: %w", raw, err)
	}

	return args, nil
}

// decisionFromAskUser builds a DecisionAsk from ask_user's arguments.
// ask_user's own schema (usecase.AskUserTool) no longer asks the model for
// a "service" argument at all - with a catalogue of several services the
// model has no reliable way to name one it never called (measured
// 2026-09-15, docs/specs/shortlisting.md: fabricated service names such as
// "approval" or "salesBundle" 500'd as ErrEndpointNotFound). The service is
// resolved from operationId instead, exactly the way decisionFromCall
// resolves a real tool call's.
//
// When operationId does not resolve to any endpoint in the catalogue - it
// is absent, or the model invented it (measured: "expense/ask_user", the
// model naming its own tool as the operation it was stuck on) - the
// Decision carries only the question, with Service, OperationID, Param and
// Options all left zero-valued. Orchestrator.ask
// (internal/usecase/orchestrator.go) turns that into a plain question
// rather than ErrEndpointNotFound: an ask is never a 500.
//
// Options is read and carried onto Decision.Options, when the operation
// did resolve, because Decision declares the field and a caller may want
// to see what the model proposed, but Orchestrator.ask never trusts it -
// it rebuilds the options a person is offered from the catalogue's own
// enum instead, since a model can list candidate values that do not exist.
func (p *Planner) decisionFromAskUser(args map[string]any) usecase.Decision {
	question := stringArg(args, "question")

	operationID := stringArg(args, "operationId")

	service, ok := resolveService(p.catalog, operationID)
	if !ok {
		return usecase.Decision{Kind: usecase.DecisionAsk, Question: question}
	}

	return usecase.Decision{
		Kind:        usecase.DecisionAsk,
		Service:     service,
		OperationID: operationID,
		Question:    question,
		Param:       stringArg(args, "param"),
		Options:     optionsArg(args, "options"),
	}
}

// decisionFromListCapabilities builds a DecisionListCapabilities from
// list_capabilities' arguments. Unlike decisionFromCall, this never
// touches resolveService: list_capabilities is not an operation id in the
// catalogue (usecase.ListCapabilitiesTool, like ask_user, is not derived
// from any service's spec), so looking it up there would only ever fail.
// Its optional "service" argument is carried as Decision.Service - not the
// service an operation belongs to, but the filter list_capabilities itself
// takes (see Decision's own doc comment, internal/usecase/planner.go).
func decisionFromListCapabilities(args map[string]any) usecase.Decision {
	return usecase.Decision{
		Kind:    usecase.DecisionListCapabilities,
		Service: stringArg(args, "service"),
	}
}

// decisionFromProposePanel builds a DecisionProposal from propose_panel's
// arguments (usecase.ProposePanelTool): service, operationId and args are
// read exactly as decisionFromCall reads a real tool call's, and
// component, chart, transform and title - all optional
// (docs/specs/proposing.md, section 3) - are read only when the model gave
// them, left zero-valued otherwise, which is exactly what
// Orchestrator.propose needs to tell "the model said nothing" from "the
// model said this" (section 4).
func decisionFromProposePanel(args map[string]any) usecase.Decision {
	return usecase.Decision{
		Kind:        usecase.DecisionProposal,
		Service:     stringArg(args, "service"),
		OperationID: stringArg(args, "operationId"),
		Args:        objectArg(args, "args"),
		Component:   domain.Component(stringArg(args, "component")),
		View:        viewArg(args),
		Title:       stringArg(args, "title"),
	}
}

// objectArg reads an object-valued argument, nil when absent or of the
// wrong type - the same defensive default stringArg and optionsArg apply
// to a model that gets a tool's schema wrong.
func objectArg(args map[string]any, name string) map[string]any {
	m, ok := args[name].(map[string]any)
	if !ok {
		return nil
	}

	return m
}

// viewArg builds propose_panel's optional view from its "chart" and
// "transform" arguments. Chart and Transform are read independently,
// mirroring domain.View's own two halves (docs/specs/dashboard.md, P1):
// the model may give either, both or neither. nil when neither was given,
// so Orchestrator.propose's catalogue fallback (proposalView) sees no
// model-supplied view at all, rather than an empty one it would otherwise
// have to tell apart from a real one.
func viewArg(args map[string]any) *domain.View {
	chart := chartArg(objectArg(args, "chart"))
	transform := transformArg(objectArg(args, "transform"))

	if chart == nil && transform == nil {
		return nil
	}

	return &domain.View{Chart: chart, Transform: transform}
}

// chartArg builds a domain.Chart from propose_panel's "chart" argument, or
// nil when the model left it out.
func chartArg(m map[string]any) *domain.Chart {
	if m == nil {
		return nil
	}

	return &domain.Chart{
		Category: stringArg(m, "category"),
		Value:    stringArg(m, "value"),
		Kind:     domain.ChartKind(stringArg(m, "kind")),
	}
}

// transformArg builds a domain.Transform from propose_panel's "transform"
// argument, or nil when the model left it out.
func transformArg(m map[string]any) *domain.Transform {
	if m == nil {
		return nil
	}

	return &domain.Transform{
		GroupBy:   stringArg(m, "groupBy"),
		Aggregate: domain.Aggregate(stringArg(m, "aggregate")),
		Field:     stringArg(m, "field"),
	}
}

// decisionFromCall builds a DecisionCall for any tool name other than
// ask_user or list_capabilities: name is the operation id (usecase.ToolsFor
// names a tool after nothing else), so the service it belongs to must be
// looked up in the catalogue.
func (p *Planner) decisionFromCall(name string, args map[string]any) (usecase.Decision, error) {
	service, ok := resolveService(p.catalog, name)
	if !ok {
		return usecase.Decision{}, fmt.Errorf("%w: %q", ErrUnknownOperation, name)
	}

	return usecase.Decision{Kind: usecase.DecisionCall, Service: service, OperationID: name, Args: args}, nil
}

// resolveService looks operationID up in catalog and returns the service
// that owns it.
//
// A tool call carries only the operation id - usecase.ToolsFor never
// qualifies a tool's Name with its service, because the tool-calling wire
// format has no separate field for it - so when two services in the
// catalogue declare the very same operation id, the call alone cannot say
// which one was meant. That is a real ambiguity, not a bug in this
// function: it is resolved by taking the first match in catalog.Endpoints
// order, which is deterministic (the catalogue is built by iterating
// configured services in a fixed order) but not necessarily correct. This
// is judged an acceptable default rather than an error, because avoiding it
// is a naming discipline on the services themselves - keep operation ids
// unique across the catalogue, which every operation id in this product so
// far already is (ListInventoryItems vs. ListAttendanceRecords, not
// List) - not something this planner can fix after the fact. See
// DECISIONS.md.
func resolveService(catalog domain.Catalog, operationID string) (string, bool) {
	for i := range catalog.Endpoints {
		if catalog.Endpoints[i].OperationID == operationID {
			return catalog.Endpoints[i].Service, true
		}
	}

	return "", false
}

// stringArg reads a string argument, defaulting to "" when absent or of
// the wrong type - a model that gets ask_user's schema wrong should not
// crash the planner, just produce a Decision Orchestrator.ask will either
// reject with ErrEndpointNotFound or degrade to a form for.
func stringArg(args map[string]any, name string) string {
	s, ok := args[name].(string)
	if !ok {
		return ""
	}

	return s
}

// optionsArg reads ask_user's "options" argument into []domain.Option. See
// decisionFromAskUser for why the result is carried but never trusted
// downstream.
func optionsArg(args map[string]any, name string) []domain.Option {
	raw, ok := args[name].([]any)
	if !ok {
		return nil
	}

	options := make([]domain.Option, 0, len(raw))

	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		options = append(options, domain.Option{Value: stringArg(m, "value"), Label: stringArg(m, "label")})
	}

	return options
}

// turnsIntro is prepended to the rendered conversation history (see
// renderTurns). English, matching systemPrompt's own convention, and
// worded so the model reads what follows as history rather than as
// something to act on again: earlier questions and the operation the
// platform chose for each, never the data that operation returned (M1,
// docs/specs/context.md section 3).
const turnsIntro = "Here is the conversation so far, oldest first. Each line is a question the user " +
	"already asked and what the platform decided to do about it - service, operation and arguments, " +
	"never the data the operation returned. Use it only to understand what \"it\", \"the same thing\" " +
	"or an unnamed service in the new question below refers to."

// buildMessages renders exactly one system message (systemPrompt, the
// selected wording.Wording.SystemPrompt) and one user message: the
// conversation so far (renderTurns), when turns is non-empty, followed by
// the current question (with answers folded in).
//
// turns is folded into the one user message rather than sent as a message
// of its own: a second "system"-role message partway through the
// conversation is not something every chat template tolerates - qwen3.5's
// (the model this platform runs against, DECISIONS.md) raises "System
// message must be at the beginning" the moment one appears anywhere but
// index 0, discovered by hand running this exact turns payload against it
// through the real /api/plan endpoint. A "user"-role message keeps
// systemPrompt itself, and the tools the request carries separately, both
// exactly as they were (M3, docs/specs/context.md section 4): only the one
// message that already changes on every request - the user turn - grows a
// prefix that changes with it.
//
// There is no assistant/tool turn here even though answers were produced
// by a previous ask_user call, and none built from turns either, even
// though each one records a call the platform actually made: each
// /api/plan request is one stateless chat completion (D8,
// docs/specs/orchestration.md - one LLM call per request), not a
// continuation of a stored conversation, so the only way either reaches
// the model at all is rendered as plain text.
func buildMessages(
	query string, answers []usecase.Answer, turns []usecase.Turn, systemPrompt string, now time.Time,
) []chat.Message {
	return []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: BuildUserContent(query, answers, turns, now)},
	}
}

// dateLine is BuildUserContent's own first line, on every planning call:
// "今日は 2026-09-16（火）です。" followed by a blank line, so the model
// has some notion of today without it ever touching systemPrompt (which
// wording's own byte-identity tests, AC-Q-101, pin unchanged) - fixing the
// dev-stack defect where a create form's date was invented outright
// (2023-10-10 for a question asked in 2026) because the model was never
// told what day it is (TODO.md, DECISIONS.md 2026-09-16 "Thirty questions
// against the real dev services").
func dateLine(now time.Time) string {
	return fmt.Sprintf("今日は %s（%s）です。\n\n", now.Format("2006-01-02"), japaneseWeekday(now.Weekday()))
}

// japaneseWeekday renders a time.Weekday as its single full-width Japanese
// character, the way a person would say it - dateLine's own "（火）".
func japaneseWeekday(d time.Weekday) string {
	return [...]string{"日", "月", "火", "水", "木", "金", "土"}[d]
}

// BuildUserContent renders dateLine first, on every call, then - when
// turns is non-empty - the conversation so far (renderTurns), then the
// current question, followed - when answers is non-empty - by every
// answer the person has already given to a previous ask_user question, so
// a resubmitted query actually uses the chosen value instead of asking
// again. Exported so internal/adapter/planner/jev's own ORCHESTRA_FILL_ENUM=jev
// Filler (docs/measurements/jev-conditions.md) can send Jev the identical
// user content the local fill would have seen for the same question,
// rather than a second, drifting copy of this rendering.
func BuildUserContent(query string, answers []usecase.Answer, turns []usecase.Turn, now time.Time) string {
	var b strings.Builder

	b.WriteString(dateLine(now))

	if len(turns) > 0 {
		b.WriteString(renderTurns(turns))
		b.WriteString("\n\n")
	}

	b.WriteString(query)

	if len(answers) > 0 {
		b.WriteString("\n\nこれまでに確認した値:\n")

		for _, a := range answers {
			fmt.Fprintf(&b, "- %s = %s\n", a.Param, a.Value)
		}

		b.WriteString("\n上記の値をそのまま使って、対応する操作を呼び出してください。")
	}

	return b.String()
}

// renderTurns renders turns as turnsIntro followed by one line each,
// naming the question, the ResultKind it resolved to, and - when a
// service was decided (absent for usecase.ResultKindNone) - the service,
// operation id and arguments the platform called it with. usecase.Turn has
// no field for the answer's own data (M1), so there is nothing here that
// could render a row of one even by mistake - see
// TestRenderTurnsNeverRendersARowOfAnyAnswer.
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

// shapeTools converts every usecase.Tool into a chat.ToolDefinition, with
// w applied to each one's description (see shapeTool).
func shapeTools(tools []usecase.Tool, w *wording.Wording) []chat.ToolDefinition {
	out := make([]chat.ToolDefinition, len(tools))
	for i, t := range tools {
		out[i] = shapeTool(&t, w)
	}

	return out
}

// shapeTool converts one usecase.Tool into the wire shape: usecase.ToolsFor
// deliberately emits a plain JSON Schema, unshaped for any one transport's
// strict-mode rules (internal/usecase/tools.go), because Task 11's JSON
// planner needs it that way too - so shaping it for tool calling's strict
// mode is this adapter's job, done here rather than in chat.Client, which
// knows nothing about usecase.Tool.
//
// OpenAI's strict function calling requires additionalProperties: false on
// every object and every declared property to be listed as required - an
// optional filter such as ListInventoryItems's "status" is not. Rather
// than fabricate a required list the model was never told about (which
// would make ListInventoryItems's own catalogue-declared "optional"
// dishonest to the model), a tool that has any optional top-level property
// is sent with strict: false. additionalProperties: false is added
// regardless of strict, top-level and on every nested object schema
// (a Response/RequestBody in this catalogue never nests an object inside
// another - see the openapi.yaml of each dummy service - so a shallow walk
// covers every case that exists today): it costs nothing on a backend that
// ignores it (llama.cpp/llama-swap, verified by hand, DECISIONS.md) and it
// is the harmless half of strict mode - it forbids an invented extra
// argument, not a real optional one - on a backend that enforces it. See
// DECISIONS.md for the writeup of this tradeoff.
//
// Description comes from descriptionFor, not t.Description directly: the
// selected wording overrides the three built-in tools' text by name, and
// renders a catalogue tool's own description from its summary (still
// t.Description - usecase.ToolsFor names it that) and its examples
// (t.Examples) via w.CatalogueTool (docs/specs/wording.md, section 3).
func shapeTool(t *usecase.Tool, w *wording.Wording) chat.ToolDefinition {
	schema := shapeSchema(t.InputSchema)
	strict := t.Strict && everyPropertyRequired(schema)

	return chat.ToolDefinition{
		Type: "function",
		Function: chat.FunctionDefinition{
			Name:        t.Name,
			Description: descriptionFor(t, w),
			Parameters:  schema,
			Strict:      &strict,
		},
	}
}

// descriptionFor returns the wording-selected description for one of the
// three built-in tools - matched by name against toolNameAskUser,
// toolNameListCapabilities and toolNameProposePanel, which mirror
// usecase.AskUserTool/ListCapabilitiesTool/ProposePanelTool's own Names -
// or, for any other tool (a catalogue operation), w.CatalogueTool applied
// to the endpoint's own summary and examples.
func descriptionFor(t *usecase.Tool, w *wording.Wording) string {
	switch t.Name {
	case toolNameAskUser:
		return w.AskUser
	case toolNameListCapabilities:
		return w.ListCapabilities
	case toolNameProposePanel:
		return w.ProposePanel
	default:
		return w.CatalogueTool(t.Description, t.Examples)
	}
}

// shapeSchema deep-copies a JSON Schema map, adding "additionalProperties":
// false to every object along the way (its own top level, its "items", and
// every one of its "properties").
func shapeSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	out := make(map[string]any, len(schema)+1)

	for k, v := range schema {
		switch k {
		case "properties":
			out[k] = shapeProperties(v)
		case "items":
			out[k] = shapeNested(v)
		default:
			out[k] = v
		}
	}

	if out["type"] == domain.SchemaTypeObject {
		out["additionalProperties"] = false
	}

	return out
}

// shapeProperties applies shapeSchema to every value of a "properties" map.
func shapeProperties(v any) any {
	props, ok := v.(map[string]any)
	if !ok {
		return v
	}

	out := make(map[string]any, len(props))
	for name, prop := range props {
		out[name] = shapeNested(prop)
	}

	return out
}

// shapeNested applies shapeSchema to one nested schema value (a
// "properties" entry or an "items" schema), tolerating a value that is not
// itself a schema map - usecase.ToolsFor never produces one, but this
// function has no way to prove that of an arbitrary map[string]any.
func shapeNested(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}

	return shapeSchema(m)
}

// everyPropertyRequired reports whether a (already-shaped) top-level
// object schema's "required" list names every one of its "properties" -
// the condition strict mode needs to be honest, per shapeTool's doc
// comment. A schema with no properties at all is vacuously true.
func everyPropertyRequired(schema map[string]any) bool {
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return true
	}

	required, ok := schema["required"].([]string)
	if !ok {
		required = nil
	}

	requiredSet := make(map[string]struct{}, len(required))
	for _, name := range required {
		requiredSet[name] = struct{}{}
	}

	for name := range props {
		if _, ok := requiredSet[name]; !ok {
			return false
		}
	}

	return true
}
