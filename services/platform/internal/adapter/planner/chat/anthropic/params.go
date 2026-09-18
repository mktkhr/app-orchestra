package anthropic

// behavior is what one Claude model needs its own request shaped around,
// beyond the fields internal/adapter/planner/chat.Request already carries:
// whether "temperature" may be sent at all, and whether "thinking" must be
// sent explicitly to turn it off.
//
// Both matter because getting them wrong is not a cosmetic difference: a
// newer model that has removed sampling parameters answers a "temperature"
// field with a 400, and a model that thinks by default spends
// Request.MaxTokens on reasoning before ever reaching an answer unless
// "thinking" is sent - the same planningMaxTokens/pickMaxTokens budgets
// this adapter still uses. Neither is a guess a live probe could have
// caught cheaply, since sending either the wrong way costs a real request
// (or, for the removed-parameter case, a 400 for nothing).
type behavior struct {
	// sendTemperature is whether Complete may put Request.Temperature (when
	// the caller set one) on the wire at all.
	sendTemperature bool
	// thinkingDisabled is whether Complete must send
	// {"type":"disabled"} in the wire body's "thinking" field - only
	// needed for a model that thinks by default (this package's own doc
	// comment).
	thinkingDisabled bool
}

// behaviorFor is the one place the per-model parameter rules live - every
// caller (toWireRequest) goes through it, so a model added or corrected
// here is corrected everywhere at once. Table-driven rather than a
// switch, so params_test.go can walk every entry once instead of one test
// per case.
//
// A local map literal, not a package-level var: harness/quality/go/golangci.yml
// enables gochecknoglobals, the same reason
// internal/adapter/planner/chat.Zero/MaxTokens return a fresh value rather
// than sharing one.
func behaviorFor(model string) behavior {
	table := map[string]behavior{
		// claude-haiku-4-5 still accepts temperature and does not think
		// unless asked - never send "thinking" to it (it rejects "effort"
		// too, but this adapter never sends that).
		"claude-haiku-4-5": {sendTemperature: true, thinkingDisabled: false},
		// claude-sonnet-5 and claude-opus-5 have had sampling parameters
		// removed (temperature is a hard 400) and think by default -
		// "thinking":{"type":"disabled"} must be sent explicitly or the
		// thinking budget eats Request.MaxTokens before an answer.
		"claude-sonnet-5": {sendTemperature: false, thinkingDisabled: true},
		"claude-opus-5":   {sendTemperature: false, thinkingDisabled: true},
		// claude-fable-5-1 has also had sampling parameters removed
		// (temperature is a hard 400); nothing in this adapter's brief says
		// it thinks by default, so "thinking" is left unsent for it, the
		// same as claude-haiku-4-5.
		"claude-fable-5-1": {sendTemperature: false, thinkingDisabled: false},
	}

	if b, ok := table[model]; ok {
		return b
	}

	// A model this package has not been told about yet gets the safer of
	// the two guesses, not the one that saves the most tokens: send
	// temperature (today's OpenAI-compatible behaviour, and the shape
	// claude-haiku-4-5 itself wants), send no "thinking" field (sending it
	// to a model that does not support it is its own way to 400).
	return behavior{sendTemperature: true, thinkingDisabled: false}
}
