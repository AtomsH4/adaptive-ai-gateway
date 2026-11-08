package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"adaptive-ai-gateway/internal/architecture"
)

func fixture(t *testing.T, sources map[string]string, rules []architecture.Rule) string {
	t.Helper()
	root := t.TempDir()
	sources["go.mod"] = "module example.test/fixture\n\ngo 1.26.0\n"
	manifest, err := json.Marshal(architecture.Manifest{SchemaVersion: 1, Module: "example.test/fixture", Rules: rules})
	if err != nil {
		t.Fatal(err)
	}
	sources["architecture.json"] = string(manifest)
	for name, content := range sources {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRejectsLayerBypassesAndUnregisteredPackages(t *testing.T) {
	rules := []architecture.Rule{
		{Package: "internal/domain", Stdlib: []string{"time"}},
		{Package: "internal/application/ports", Internal: []string{"internal/domain"}, Stdlib: []string{"context"}},
		{Package: "internal/application", Internal: []string{"internal/domain", "internal/application/ports"}, Stdlib: []string{"context"}},
	}
	cases := []struct{ name, path, source, reason string }{
		{"domain network", "internal/domain/x.go", "package domain\nimport _ \"net/http\"\n", "outside layer contract"},
		{"application adapter bypass", "internal/application/x.go", "package application\nimport _ \"example.test/fixture/internal/adapter/engine\"\n", "forbidden internal dependency"},
		{"port reverse dependency", "internal/application/ports/x.go", "package ports\nimport _ \"example.test/fixture/internal/application\"\n", "forbidden internal dependency"},
		{"hidden external dependency", "internal/domain/x.go", "package domain\nimport _ \"example.com/vendor/runtime\"\n", "not approved"},
		{"new unregistered layer", "internal/misc/x.go", "package misc\n", "unregistered"},
		{"non-host platform file", "internal/domain/x_linux.go", "//go:build linux\n\npackage domain\nimport _ \"os/exec\"\n", "outside layer contract"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t, map[string]string{tc.path: tc.source}, rules)
			violations, err := architecture.Check(root, "architecture.json")
			if err != nil {
				t.Fatal(err)
			}
			if len(violations) != 1 || !strings.Contains(violations[0].Reason, tc.reason) {
				t.Fatalf("got %+v", violations)
			}
		})
	}
}

func TestAllowsDeclaredDependenciesButRejectsCycles(t *testing.T) {
	rules := []architecture.Rule{
		{Package: "internal/a", Internal: []string{"internal/b"}, Stdlib: []string{"context"}},
		{Package: "internal/b", Internal: []string{"internal/a"}, Stdlib: []string{"context"}},
	}
	root := fixture(t, map[string]string{
		"internal/a/x.go": "package a\nimport _ \"example.test/fixture/internal/b\"\n",
		"internal/b/x.go": "package b\nimport _ \"context\"\n",
	}, rules)
	if violations, err := architecture.Check(root, "architecture.json"); err != nil || len(violations) != 0 {
		t.Fatalf("valid graph rejected: %+v %v", violations, err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal/b/x.go"), []byte("package b\nimport _ \"example.test/fixture/internal/a\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	violations, err := architecture.Check(root, "architecture.json")
	if err != nil || len(violations) == 0 || !strings.Contains(violations[0].Reason, "cycle") {
		t.Fatalf("cycle missed: %+v %v", violations, err)
	}
}

func TestRepositoryProductionImports(t *testing.T) {
	violations, err := architecture.Check("../..", "architecture/dependencies.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation.String())
	}
}

func TestRepositoryHTTPBoundaryRejectsOutboundAuthority(t *testing.T) {
	data, err := os.ReadFile("../../architecture/dependencies.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest architecture.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, dependency := range []string{"internal/application/ports", "internal/application", "internal/adapter/llamacpp"} {
		t.Run(dependency, func(t *testing.T) {
			root := fixture(t, map[string]string{
				"internal/adapter/http/x.go": "package http\nimport _ \"example.test/fixture/" + dependency + "\"\n",
			}, manifest.Rules)
			violations, err := architecture.Check(root, "architecture.json")
			if err != nil || len(violations) != 1 || !strings.Contains(violations[0].Reason, "forbidden internal dependency") {
				t.Fatalf("HTTP acquired outbound authority: %+v %v", violations, err)
			}
		})
	}
}
