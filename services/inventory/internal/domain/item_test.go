package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/inventory/internal/domain"
)

func TestStatusValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status domain.Status
		want   bool
	}{
		{"allocated", domain.StatusAllocated, true},
		{"staged", domain.StatusStaged, true},
		{"quarantined", domain.StatusQuarantined, true},
		{"consigned", domain.StatusConsigned, true},
		{"unknown", domain.Status("unknown"), false},
		{"empty", domain.Status(""), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, c.status.Valid())
		})
	}
}
