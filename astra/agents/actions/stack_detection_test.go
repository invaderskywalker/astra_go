package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectRepositoryStackFindsPolyglotEvidenceAndCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ASTRA_DATA_DIR", filepath.Join(t.TempDir(), "astra"))
	files := map[string]string{
		"go.mod":                "module example.test/app\n\ngo 1.22\n",
		"package.json":          `{"scripts":{"test":"vitest","build":"vite build"},"dependencies":{"react":"^18.0.0","vite":"^5.0.0"},"devDependencies":{"typescript":"^5.0.0"}}`,
		"pnpm-lock.yaml":        "lockfileVersion: 9\n",
		"pyproject.toml":        "[project]\nname = 'worker'\n",
		"tests/test_worker.py":  "def test_worker():\n    assert True\n",
		"infra/main.tf":         "terraform {}\n",
		"Dockerfile":            "FROM golang:1.22\n",
		"native/CMakeLists.txt": "cmake_minimum_required(VERSION 3.20)\n",
		"__pycache__/bad.pyc":   "package garbage\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	registry := NewDataActionsForSessionAt(nil, 1, "stack-test", root)
	result := registry.DetectRepositoryStack(DetectRepositoryStackParams{MaxFiles: 100})
	if !result.Success {
		t.Fatalf("stack detection failed: %#v", result)
	}
	report, ok := result.Diagnostics.(RepositoryStackReport)
	if !ok {
		t.Fatalf("unexpected stack report type: %#v", result.Diagnostics)
	}
	for _, want := range []string{"Go", "Node.js", "TypeScript", "Python", "Docker", "Terraform", "C/C++"} {
		if !containsString(report.Ecosystems, want) {
			t.Fatalf("ecosystem %q missing: %#v", want, report.Ecosystems)
		}
	}
	for _, want := range []string{"React", "Vite"} {
		if !containsString(report.Frameworks, want) {
			t.Fatalf("framework %q missing: %#v", want, report.Frameworks)
		}
	}
	for _, want := range []string{"go test ./...", "pnpm run test", "python -m pytest", "terraform validate"} {
		if !containsString(report.ValidationCommands, want) {
			t.Fatalf("validation command %q missing: %#v", want, report.ValidationCommands)
		}
	}
	if report.ProjectType != "polyglot/multi-stack" || report.FilesScanned != len(files)-1 {
		t.Fatalf("unexpected project classification/count: %#v", report)
	}
	if !containsString(report.Modules, ".") || !containsString(report.Modules, "native") {
		t.Fatalf("module roots missing: %#v", report.Modules)
	}
	for _, manifest := range report.Manifests {
		if strings.Contains(manifest.Path, "__pycache__") {
			t.Fatalf("generated cache appeared as manifest: %#v", manifest)
		}
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
