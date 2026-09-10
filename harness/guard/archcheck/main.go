// Command archcheck validates the layer dependencies of every backend service
// against harness/quality/architecture.json.
//
// It is part of the Repository Harness and is deliberately independent of
// golangci-lint: the architecture must stay enforceable even if the lint
// configuration is changed. Run it from the repository root.
//
// A service is a directory under the policy's dir that holds a go.mod. Nothing
// else declares the list, so a new service is checked from the moment it exists.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	exitFailure   = 1
	defaultPolicy = "harness/quality/architecture.json"
)

// errPolicyViolation signals that the source tree breaks the architecture policy.
var errPolicyViolation = errors.New("architecture policy violated")

// errIncompletePolicy signals that the policy file is missing required fields.
var errIncompletePolicy = errors.New("services dir, module prefix or layers missing")

// layer is one entry of the service layer list.
type layer struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// policy is the subset of harness/quality/architecture.json that this command
// reads. One declaration covers every service: they share a layer structure.
type policy struct {
	Services struct {
		Dir          string  `json:"dir"`
		ModulePrefix string  `json:"modulePrefix"`
		Layers       []layer `json:"layers"`
	} `json:"services"`
}

// report is what one pass over every service produced. It exists because the
// lint policy allows neither three unnamed results (gocritic) nor named ones
// (nonamedreturns), which is a fair push towards saying what the pair means.
type report struct {
	violations []string
	packages   int
}

// goPackage is the subset of `go list -json` output that this command reads.
type goPackage struct {
	ImportPath   string   `json:"ImportPath"`
	Imports      []string `json:"Imports"`
	TestImports  []string `json:"TestImports"`
	XTestImports []string `json:"XTestImports"`
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "archcheck: %v\n", err)
		os.Exit(exitFailure)
	}
}

// run loads the policy and reports every violation across every service.
func run(ctx context.Context) error {
	root, err := repositoryRoot()
	if err != nil {
		return err
	}

	pol, err := loadPolicy(filepath.Join(root, defaultPolicy))
	if err != nil {
		return err
	}

	services, err := discoverServices(filepath.Join(root, pol.Services.Dir))
	if err != nil {
		return err
	}

	if len(services) == 0 {
		fmt.Printf("archcheck: no services under %s yet\n", pol.Services.Dir)

		return nil
	}

	found, err := checkServices(ctx, root, pol, services)
	if err != nil {
		return err
	}

	if len(found.violations) == 0 {
		fmt.Printf("archcheck: %d service(s), %d packages, %d layers, no violations\n",
			len(services), found.packages, len(pol.Services.Layers))

		return nil
	}

	sort.Strings(found.violations)

	for _, violation := range found.violations {
		fmt.Fprintln(os.Stderr, violation)
	}

	return fmt.Errorf("%w: %d violation(s)", errPolicyViolation, len(found.violations))
}

// checkServices runs the policy over each service.
func checkServices(ctx context.Context, root string, pol policy, services []string) (report, error) {
	var found report

	for _, service := range services {
		dir := filepath.Join(root, pol.Services.Dir, service)

		listed, err := listPackages(ctx, dir)
		if err != nil {
			return report{}, err
		}

		module := pol.Services.ModulePrefix + "/" + service
		found.packages += len(listed)
		found.violations = append(found.violations, check(pol, module, listed)...)
	}

	return found, nil
}

// discoverServices returns every directory under dir holding a go.mod, sorted.
func discoverServices(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)

	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	services := make([]string, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if _, statErr := os.Stat(filepath.Join(dir, entry.Name(), "go.mod")); statErr != nil {
			continue
		}

		services = append(services, entry.Name())
	}

	sort.Strings(services)

	return services, nil
}

// repositoryRoot walks up from the working directory until it finds the policy file.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	for {
		if _, statErr := os.Stat(filepath.Join(dir, defaultPolicy)); statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("locate %s: %w", defaultPolicy, os.ErrNotExist)
		}

		dir = parent
	}
}

// loadPolicy reads and decodes the architecture policy.
func loadPolicy(file string) (policy, error) {
	raw, err := os.ReadFile(file) //nolint:gosec // the policy path is a constant
	if err != nil {
		return policy{}, fmt.Errorf("read %s: %w", file, err)
	}

	var pol policy
	if err := json.Unmarshal(raw, &pol); err != nil {
		return policy{}, fmt.Errorf("parse %s: %w", file, err)
	}

	if pol.Services.Dir == "" || pol.Services.ModulePrefix == "" || len(pol.Services.Layers) == 0 {
		return policy{}, fmt.Errorf("parse %s: %w", file, errIncompletePolicy)
	}

	return pol, nil
}

// listPackages asks the go tool to describe every package of one module.
func listPackages(ctx context.Context, dir string) ([]goPackage, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "./...")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list in %s: %w", dir, err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(out)))

	var packages []goPackage

	for decoder.More() {
		var pkg goPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// check returns a human readable message for every policy violation found in
// one service, identified by its module path.
func check(pol policy, module string, packages []goPackage) []string {
	var violations []string

	for _, pkg := range packages {
		rank, ok := rankOf(pol, module, pkg.ImportPath)
		if !ok {
			violations = append(violations, fmt.Sprintf(
				"%s: package does not belong to any declared layer (%s)",
				pkg.ImportPath, layerNames(pol)))

			continue
		}

		for _, imported := range allImports(&pkg) {
			violations = append(violations, checkImport(pol, module, pkg.ImportPath, rank, imported)...)
		}
	}

	return violations
}

// checkImport validates a single import edge.
func checkImport(pol policy, module, importer string, importerRank int, imported string) []string {
	if !strings.HasPrefix(imported, module+"/") {
		return nil
	}

	importedRank, ok := rankOf(pol, module, imported)
	if !ok {
		return []string{fmt.Sprintf(
			"%s: imports %s which does not belong to any declared layer",
			importer, imported)}
	}

	if importedRank > importerRank {
		return []string{fmt.Sprintf(
			"%s (%s) imports %s (%s): dependencies must point inwards",
			importer, pol.Services.Layers[importerRank].Name,
			imported, pol.Services.Layers[importedRank].Name)}
	}

	return nil
}

// allImports merges the production and test imports of a package.
func allImports(pkg *goPackage) []string {
	merged := make([]string, 0, len(pkg.Imports)+len(pkg.TestImports)+len(pkg.XTestImports))
	merged = append(merged, pkg.Imports...)
	merged = append(merged, pkg.TestImports...)
	merged = append(merged, pkg.XTestImports...)

	return merged
}

// rankOf returns the index of the layer an import path belongs to.
func rankOf(pol policy, module, importPath string) (int, bool) {
	rel, ok := strings.CutPrefix(importPath, module)
	if !ok {
		return 0, false
	}

	rel = strings.TrimPrefix(rel, "/")

	for index, entry := range pol.Services.Layers {
		if rel == entry.Path || strings.HasPrefix(rel, entry.Path+"/") || matchesDir(rel, entry.Path) {
			return index, true
		}
	}

	return 0, false
}

// matchesDir reports whether rel sits inside dir, comparing cleaned paths.
func matchesDir(rel, dir string) bool {
	return path.Clean(rel) == path.Clean(dir)
}

// layerNames renders the declared layer names for error messages.
func layerNames(pol policy) string {
	names := make([]string, 0, len(pol.Services.Layers))
	for _, entry := range pol.Services.Layers {
		names = append(names, entry.Name)
	}

	return strings.Join(names, ", ")
}
