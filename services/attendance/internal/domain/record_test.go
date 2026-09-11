package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/attendance/internal/domain"
)

func TestKindValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind domain.Kind
		want bool
	}{
		{"deemed", domain.KindDeemed, true},
		{"substitute", domain.KindSubstitute, true},
		{"compensatory", domain.KindCompensatory, true},
		{"on_call", domain.KindOnCall, true},
		{"unknown", domain.Kind("unknown"), false},
		{"empty", domain.Kind(""), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, c.kind.Valid())
		})
	}
}
