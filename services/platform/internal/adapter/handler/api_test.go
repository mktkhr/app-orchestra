package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/handler"
	"github.com/mktkhr/app-orchestra/services/platform/internal/adapter/openapi"
)

func TestNewAPIComposesEveryTag(t *testing.T) {
	api := handler.NewAPI(handler.NewHealth(), handler.NewPlan(nil), handler.NewInvoke(nil), handler.NewWorkspace(nil))

	var iface openapi.StrictServerInterface = api
	assert.NotNil(t, iface)
}
