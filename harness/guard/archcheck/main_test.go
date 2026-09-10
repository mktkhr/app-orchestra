package main

import (
	"strings"
	"testing"
)

// testModule is the module path of the imaginary service under test.
const testModule = "example.com/app"

// testPolicy mirrors the services section of harness/quality/architecture.json.
func testPolicy() policy {
	var pol policy

	pol.Services.Dir = "services"
	pol.Services.ModulePrefix = "example.com"
	pol.Services.Layers = []layer{
		{Name: "domain", Path: "internal/domain"},
		{Name: "usecase", Path: "internal/usecase"},
		{Name: "adapter", Path: "internal/adapter"},
		{Name: "infra", Path: "internal/infra"},
		{Name: "cmd", Path: "cmd"},
	}

	return pol
}

func TestCheckAcceptsInwardDependencies(t *testing.T) {
	t.Parallel()

	packages := []goPackage{
		{ImportPath: "example.com/app/internal/domain"},
		{ImportPath: "example.com/app/internal/usecase", Imports: []string{"example.com/app/internal/domain", "context"}},
		{ImportPath: "example.com/app/internal/adapter/handler", Imports: []string{"example.com/app/internal/usecase"}},
		{ImportPath: "example.com/app/cmd/api", Imports: []string{"example.com/app/internal/infra/httpserver"}},
		{ImportPath: "example.com/app/internal/infra/httpserver", Imports: []string{"example.com/app/internal/adapter/handler"}},
	}

	if violations := check(testPolicy(), testModule, packages); len(violations) != 0 {
		t.Fatalf("check() = %v, want no violations", violations)
	}
}

func TestCheckRejectsOutwardDependencies(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		pkg  goPackage
		want string
	}{
		"domain imports usecase": {
			pkg:  goPackage{ImportPath: "example.com/app/internal/domain", Imports: []string{"example.com/app/internal/usecase"}},
			want: "dependencies must point inwards",
		},
		"usecase imports adapter": {
			pkg:  goPackage{ImportPath: "example.com/app/internal/usecase", Imports: []string{"example.com/app/internal/adapter/handler"}},
			want: "dependencies must point inwards",
		},
		"adapter imports infra through a test": {
			pkg:  goPackage{ImportPath: "example.com/app/internal/adapter/handler", TestImports: []string{"example.com/app/internal/infra/httpserver"}},
			want: "dependencies must point inwards",
		},
		"unclassified package": {
			pkg:  goPackage{ImportPath: "example.com/app/internal/util"},
			want: "does not belong to any declared layer",
		},
		"import of an unclassified package": {
			pkg:  goPackage{ImportPath: "example.com/app/internal/domain", Imports: []string{"example.com/app/pkg/helper"}},
			want: "does not belong to any declared layer",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			violations := check(testPolicy(), testModule, []goPackage{tc.pkg})
			if len(violations) == 0 {
				t.Fatalf("check() reported no violation, want one containing %q", tc.want)
			}

			if !strings.Contains(strings.Join(violations, "\n"), tc.want) {
				t.Errorf("check() = %v, want a violation containing %q", violations, tc.want)
			}
		})
	}
}

func TestCheckIgnoresExternalImports(t *testing.T) {
	t.Parallel()

	pkg := goPackage{
		ImportPath: "example.com/app/internal/domain",
		Imports:    []string{"context", "errors", "github.com/some/library"},
	}

	if violations := check(testPolicy(), testModule, []goPackage{pkg}); len(violations) != 0 {
		t.Fatalf("check() = %v, want no violations", violations)
	}
}
