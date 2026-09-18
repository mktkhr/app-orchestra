package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
)

// stopReasonMaxTokens and stopReasonRefusal are the two Messages API
// "stop_reason" values fromWireResponse treats specially: the first maps
// onto chat.FinishReasonLength, the same truncation signal toolcall.Planner
// and pick.Picker already check for (chat.FinishReasonLength's own doc
// comment); the second - a model refusing to answer at all, a value the
// OpenAI-compatible wire format this package's sibling speaks has no
// equivalent for - becomes an error instead of a Response, since neither
// caller has anywhere to route a refusal through Decision.
const (
	stopReasonMaxTokens = "max_tokens"
	stopReasonRefusal   = "refusal"
)

// ErrRefused is wrapped into the error fromWireResponse returns when the
// model's stop_reason is "refusal".
var ErrRefused = errors.New("anthropic: model refused the request")

// wireContentBlock is one entry of wireResponseBody's "content": a "text"
// block (Text set) or a "tool_use" block (ID, Name and Input set) - the
// only two block types this adapter reads back, since neither planner
// stage's request ever asks for anything else (no extended thinking is
// ever left on - params.go's thinkingDisabled - so no "thinking" block is
// ever expected either).
type wireContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// wireResponseBody is the JSON POST /v1/messages answers with.
type wireResponseBody struct {
	Content    []wireContentBlock `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// fromWireResponse maps one Messages API response onto the same
// chat.Response shape internal/adapter/planner/chat.Client's own do
// returns, so toolcall.Planner and pick.Picker read it exactly the same
// way regardless of which transport answered: every "text" block's Text is
// concatenated onto Message.Content (both existing callers only ever send
// one), every "tool_use" block becomes one chat.ToolCall with its Input
// re-marshalled as the same JSON-string Arguments the OpenAI wire format
// carries (toolcall.decodeArguments unmarshals it the same way either
// transport produced it), and "stop_reason":"max_tokens" becomes
// chat.FinishReasonLength - any other stop_reason ("end_turn", "tool_use",
// ...) passes through unchanged, since neither caller compares FinishReason
// to anything else.
func fromWireResponse(wire *wireResponseBody) (chat.Response, error) {
	if wire.StopReason == stopReasonRefusal {
		return chat.Response{}, fmt.Errorf("%w: %s", ErrRefused, refusalDetail(wire.Content))
	}

	var textParts []string

	var toolCalls []chat.ToolCall

	for _, block := range wire.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, chat.ToolCall{
				ID:       block.ID,
				Type:     "function",
				Function: chat.FunctionCall{Name: block.Name, Arguments: string(block.Input)},
			})
		}
	}

	finishReason := wire.StopReason
	if finishReason == stopReasonMaxTokens {
		finishReason = chat.FinishReasonLength
	}

	return chat.Response{
		Message: chat.Message{
			Role:      "assistant",
			Content:   strings.Join(textParts, ""),
			ToolCalls: toolCalls,
		},
		FinishReason: finishReason,
		Usage:        chat.Usage{CompletionTokens: wire.Usage.OutputTokens},
	}, nil
}

// refusalDetail names what a refusal looked like - the first non-empty
// "text" block's content, or a fixed string when the model gave none - so
// ErrRefused says something more useful than the bare stop_reason.
func refusalDetail(content []wireContentBlock) string {
	for _, b := range content {
		if b.Type == "text" && b.Text != "" {
			return b.Text
		}
	}

	return "no detail given"
}
