package anthropic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/chat/anthropic"
)

// TestPerModelParameterRules is the "one place, table-driven and tested"
// per-model rule set the task brief asks for: haiku-4-5 gets temperature
// and no thinking field; sonnet-5 and opus-5 get no temperature and
// thinking:disabled; fable-5-1 gets no temperature and no thinking field;
// an unknown model id falls back to the safer default (temperature sent,
// no thinking field) rather than erroring.
func TestPerModelParameterRules(t *testing.T) {
	tests := []struct {
		model           string
		wantTemperature bool
		wantThinking    bool
	}{
		{model: "claude-haiku-4-5", wantTemperature: true, wantThinking: false},
		{model: "claude-sonnet-5", wantTemperature: false, wantThinking: true},
		{model: "claude-opus-5", wantTemperature: false, wantThinking: true},
		{model: "claude-fable-5-1", wantTemperature: false, wantThinking: false},
		{model: "some-unreleased-model", wantTemperature: true, wantThinking: false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			temp := 0.0
			maxTokens := 1024

			body, err := anthropic.BuildRequestBody(tt.model, &chat.Request{
				Messages:    []chat.Message{{Role: "user", Content: "hi"}},
				Temperature: &temp,
				MaxTokens:   &maxTokens,
			})
			require.NoError(t, err)

			decoded := decodeBody(t, body)

			_, hasTemperature := decoded["temperature"]
			assert.Equal(t, tt.wantTemperature, hasTemperature, "temperature presence for %s", tt.model)

			thinking, hasThinking := decoded["thinking"]
			assert.Equal(t, tt.wantThinking, hasThinking, "thinking presence for %s", tt.model)

			if tt.wantThinking {
				thinkingMap, ok := thinking.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "disabled", thinkingMap["type"])
			}
		})
	}
}
