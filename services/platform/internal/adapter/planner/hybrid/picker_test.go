package hybrid_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/planner/hybrid"
	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
	"github.com/mktkhr/app-orchestra/services/platform/internal/usecase"
)

// fakePicker is a usecase.Picker double: pick supplies the answer (and
// error) this fake returns, and calls counts how many times Pick was
// invoked - the one assertion this whole suite leans on to prove the
// unused side of a fallback decision was never called at all.
type fakePicker struct {
	pick  func(ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog) (usecase.Pick, error)
	calls int
}

func (f *fakePicker) Pick(
	ctx context.Context, query string, answers []usecase.Answer, turns []usecase.Turn, shortlist domain.Catalog,
) (usecase.Pick, error) {
	f.calls++

	return f.pick(ctx, query, answers, turns, shortlist)
}

var _ usecase.Picker = (*fakePicker)(nil)

func fixedPick(result usecase.Pick, err error) func(
	context.Context, string, []usecase.Answer, []usecase.Turn, domain.Catalog,
) (usecase.Pick, error) {
	return func(context.Context, string, []usecase.Answer, []usecase.Turn, domain.Catalog) (usecase.Pick, error) {
		return result, err
	}
}

func TestPickUsesJevWhenConfidentCatalogueOperationAndNeverCallsLocal(t *testing.T) {
	jevPick := usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems", Confidence: 0.9}
	jev := &fakePicker{pick: fixedPick(jevPick, nil)}
	local := &fakePicker{pick: fixedPick(usecase.Pick{Kind: usecase.PickNone}, nil)}

	p := hybrid.New(jev, local)

	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, jevPick, got)
	assert.Equal(t, 0, local.calls, "local picker must not be called when Jev is confident")
}

func TestPickFallsBackToLocalOnLowConfidence(t *testing.T) {
	jevPick := usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems", Confidence: 0.4}
	localPick := usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems"}
	jev := &fakePicker{pick: fixedPick(jevPick, nil)}
	local := &fakePicker{pick: fixedPick(localPick, nil)}

	p := hybrid.New(jev, local, hybrid.WithThreshold(0.7))

	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, localPick, got)
	assert.Equal(t, 1, local.calls)
}

func TestPickFallsBackToLocalOnBuiltinRegardlessOfConfidence(t *testing.T) {
	jevPick := usecase.Pick{Kind: usecase.PickNone, Confidence: 0.99}
	localPick := usecase.Pick{Kind: usecase.PickListCapabilities}
	jev := &fakePicker{pick: fixedPick(jevPick, nil)}
	local := &fakePicker{pick: fixedPick(localPick, nil)}

	p := hybrid.New(jev, local)

	got, err := p.Pick(t.Context(), "何ができる？", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, localPick, got)
	assert.Equal(t, 1, local.calls)
}

func TestPickFallsBackToLocalOnJevError(t *testing.T) {
	localPick := usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems"}
	jev := &fakePicker{pick: fixedPick(usecase.Pick{}, errors.New("jev unreachable"))}
	local := &fakePicker{pick: fixedPick(localPick, nil)}

	p := hybrid.New(jev, local)

	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, localPick, got)
	assert.Equal(t, 1, local.calls)
}

func TestPickFallsBackToLocalOnTimeoutWithinTestBudget(t *testing.T) {
	blockFor := 500 * time.Millisecond
	localPick := usecase.Pick{Kind: usecase.PickOperation, Service: "inventory", OperationID: "listInventoryItems"}

	jev := &fakePicker{pick: func(
		ctx context.Context, _ string, _ []usecase.Answer, _ []usecase.Turn, _ domain.Catalog,
	) (usecase.Pick, error) {
		select {
		case <-time.After(blockFor):
			return usecase.Pick{Kind: usecase.PickOperation, OperationID: "tooLate"}, nil
		case <-ctx.Done():
			return usecase.Pick{}, ctx.Err()
		}
	}}
	local := &fakePicker{pick: fixedPick(localPick, nil)}

	p := hybrid.New(jev, local, hybrid.WithJevTimeout(30*time.Millisecond))

	start := time.Now()
	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, localPick, got)
	assert.Equal(t, 1, local.calls)
	assert.Less(t, elapsed, blockFor, "hybrid must fall back well before the blocked Jev call would return")
}

func TestPickReturnsNoneWithoutErrorWhenBothFail(t *testing.T) {
	jev := &fakePicker{pick: fixedPick(usecase.Pick{}, errors.New("jev down"))}
	local := &fakePicker{pick: fixedPick(usecase.Pick{}, errors.New("local down too"))}

	p := hybrid.New(jev, local)

	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, usecase.Pick{Kind: usecase.PickNone}, got)
}

func TestNewDefaultsTimeoutAndThresholdWhenOptionsGiveZeroOrNone(t *testing.T) {
	// A zero-duration WithJevTimeout must not collapse Pick's own
	// deadline to "already expired" - New keeps defaultJevTimeout
	// instead, so a confident Jev answer still wins.
	jevPick := usecase.Pick{Kind: usecase.PickOperation, OperationID: "listInventoryItems", Confidence: 0.9}
	jev := &fakePicker{pick: fixedPick(jevPick, nil)}
	local := &fakePicker{pick: fixedPick(usecase.Pick{Kind: usecase.PickNone}, nil)}

	p := hybrid.New(jev, local, hybrid.WithJevTimeout(0))

	got, err := p.Pick(t.Context(), "在庫を見せて", nil, nil, domain.Catalog{})
	require.NoError(t, err)
	assert.Equal(t, jevPick, got)
	assert.Equal(t, 0, local.calls)
}
