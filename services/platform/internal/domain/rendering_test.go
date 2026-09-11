package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mktkhr/app-orchestra/services/platform/internal/domain"
)

func TestRenderHintOverridesEverything(t *testing.T) {
	e := domain.Endpoint{
		UIHint:      domain.ComponentDetail,
		RequestBody: &domain.Schema{Type: "object"},
		Response: &domain.Schema{
			Type:  "array",
			Items: &domain.Schema{Type: "object"},
		},
	}

	assert.Equal(t, domain.ComponentDetail, domain.Render(&e))
}

func TestRenderRequestBodyMeansForm(t *testing.T) {
	e := domain.Endpoint{
		RequestBody: &domain.Schema{Type: "object", Properties: map[string]domain.Schema{
			"name": {Type: "string"},
		}},
		// A request body wins over what the response would otherwise render as.
		Response: &domain.Schema{
			Type:  "array",
			Items: &domain.Schema{Type: "object"},
		},
	}

	assert.Equal(t, domain.ComponentForm, domain.Render(&e))
}

func TestRenderBareObjectArrayMeansTable(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{
			Type:  "array",
			Items: &domain.Schema{Type: "object"},
		},
	}

	assert.Equal(t, domain.ComponentTable, domain.Render(&e))
}

func TestRenderWrappedArrayMeansTable(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{
			Type: "object",
			Properties: map[string]domain.Schema{
				"items": {
					Type:  "array",
					Items: &domain.Schema{Type: "object"},
				},
				"total": {Type: "integer"},
			},
		},
	}

	assert.Equal(t, domain.ComponentTable, domain.Render(&e))
}

func TestRenderSingleObjectMeansDetail(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{
			Type: "object",
			Properties: map[string]domain.Schema{
				"id":   {Type: "string"},
				"name": {Type: "string"},
			},
		},
	}

	assert.Equal(t, domain.ComponentDetail, domain.Render(&e))
}

func TestRenderArrayOfScalarsIsNotATable(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{
			Type:  "array",
			Items: &domain.Schema{Type: "string"},
		},
	}

	assert.Equal(t, domain.Component(""), domain.Render(&e))
}

func TestRenderObjectWithTwoArrayPropertiesIsNotAWrapper(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{
			Type: "object",
			Properties: map[string]domain.Schema{
				"items":  {Type: "array", Items: &domain.Schema{Type: "object"}},
				"errors": {Type: "array", Items: &domain.Schema{Type: "object"}},
			},
		},
	}

	assert.Equal(t, domain.ComponentDetail, domain.Render(&e))
}

func TestRenderNilResponseAndNoHintHasNoComponent(t *testing.T) {
	e := domain.Endpoint{}

	assert.Equal(t, domain.Component(""), domain.Render(&e))
}

func TestRenderScalarResponseHasNoComponent(t *testing.T) {
	e := domain.Endpoint{
		Response: &domain.Schema{Type: "string"},
	}

	assert.Equal(t, domain.Component(""), domain.Render(&e))
}

func TestRenderResultIgnoresTheRequestBody(t *testing.T) {
	// The same endpoint Render draws as a form: once it has been called,
	// the request body says nothing about the answer that came back.
	e := domain.Endpoint{
		RequestBody: &domain.Schema{Type: "object", Properties: map[string]domain.Schema{
			"name": {Type: "string"},
		}},
		Response: &domain.Schema{Type: "object", Properties: map[string]domain.Schema{
			"id": {Type: "string"},
		}},
	}

	assert.Equal(t, domain.ComponentForm, domain.Render(&e))
	assert.Equal(t, domain.ComponentDetail, domain.RenderResult(&e))
}

func TestRenderResultHintStillWins(t *testing.T) {
	e := domain.Endpoint{
		UIHint:      domain.ComponentTable,
		RequestBody: &domain.Schema{Type: "object"},
		Response:    &domain.Schema{Type: "object"},
	}

	assert.Equal(t, domain.ComponentTable, domain.RenderResult(&e))
}

func TestRenderResultWithNoResponseHasNoComponent(t *testing.T) {
	e := domain.Endpoint{
		RequestBody: &domain.Schema{Type: "object"},
	}

	assert.Empty(t, domain.RenderResult(&e))
}
