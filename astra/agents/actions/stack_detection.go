package actions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"astra/astra/agents/workspace"
)

// DetectRepositoryStackParams asks for bounded, metadata-first repository
// orientation. It never reads ordinary source bodies.
type DetectRepositoryStackParams struct {
	Path     string `json:"path,omitempty"`
	MaxFiles int    `json:"max_files,omitempty"`
}

type StackManifest struct {
	Path       string   `json:"path"`
	Kind       string   `json:"kind"`
	Ecosystems []string `json:"ecosystems,omitempty"`
	Details    []string `json:"details,omitempty"`
}

type RepositoryStackReport struct {
	Root               string          `json:"root"`
	ProjectType        string          `json:"project_type"`
	FilesScanned       int             `json:"files_scanned"`
	Languages          map[string]int  `json:"languages,omitempty"`
	Ecosystems         []string        `json:"ecosystems,omitempty"`
	Frameworks         []string        `json:"frameworks,omitempty"`
	Manifests          []StackManifest `json:"manifests,omitempty"`
	Modules            []string        `json:"modules,omitempty"`
	TestSignals        []string        `json:"test_signals,omitempty"`
	ValidationCommands []string        `json:"validation_commands,omitempty"`
	IgnoredGenerated   []string        `json:"ignored_generated,omitempty"`
	Warnings           []string        `json:"warnings,omitempty"`
}

const maxStackMarkerBytes = 512 * 1024

func (a *DataActions) DetectRepositoryStack(params DetectRepositoryStackParams) ActionResult {
	requested := strings.TrimSpace(params.Path)
	if requested == "" {
		requested = "."
	}
	root, err := a.workspace.ResolvePath(requested)
	if err != nil {
		return ActionResult{Success: false, Error: fmt.Sprintf("resolve repository path: %v", err)}
	}
	info, err := os.Stat(root)
	if err != nil {
		return ActionResult{Success: false, Error: fmt.Sprintf("stat repository path: %v", err)}
	}
	if !info.IsDir() {
		return ActionResult{Success: false, Error: "repository stack detection requires a directory"}
	}
	if params.MaxFiles <= 0 || params.MaxFiles > 10000 {
		params.MaxFiles = 2000
	}

	report := RepositoryStackReport{Root: root, Languages: map[string]int{}}
	ecosystems := map[string]bool{}
	frameworks := map[string]bool{}
	commands := map[string]bool{}
	manifestSeen := map[string]bool{}
	moduleRoots := map[string]bool{}
	testSignals := map[string]bool{}
	ignored := map[string]bool{}

	addManifest := func(path, kind string, values ...string) {
		rel, _ := filepath.Rel(a.workspace.Root, path)
		rel = filepath.ToSlash(rel)
		if manifestSeen[rel] {
			return
		}
		manifestSeen[rel] = true
		moduleRoot := filepath.ToSlash(filepath.Dir(rel))
		if moduleRoot == "" {
			moduleRoot = "."
		}
		moduleRoots[moduleRoot] = true
		manifest := StackManifest{Path: rel, Kind: kind}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				manifest.Ecosystems = append(manifest.Ecosystems, value)
			}
		}
		report.Manifests = append(report.Manifests, manifest)
	}
	addLanguage := func(language string) {
		if language != "" && language != "text" {
			report.Languages[language]++
		}
	}
	addTestSignal := func(signal string) { testSignals[signal] = true }

	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && workspace.ShouldSkipGeneratedDirectory(entry.Name()) {
				ignored[entry.Name()] = true
				return filepath.SkipDir
			}
			return nil
		}
		if report.FilesScanned >= params.MaxFiles {
			return filepath.SkipAll
		}
		if workspace.ShouldSkipGeneratedFile(entry.Name()) {
			ignored[entry.Name()] = true
			return nil
		}
		report.FilesScanned++
		name := strings.ToLower(entry.Name())
		rel, _ := filepath.Rel(root, path)
		language := languageForPath(rel)
		addLanguage(language)
		if isTestPath(rel) {
			addTestSignal(filepath.ToSlash(rel))
		}
		switch name {
		case "go.mod", "go.work":
			ecosystems["Go"] = true
			addManifest(path, "Go module", "Go")
			commands["go test ./..."] = true
			commands["go build ./..."] = true
		case "package.json":
			ecosystems["Node.js"] = true
			addManifest(path, "JavaScript/TypeScript package", "Node.js")
			inspectPackageManifest(path, ecosystems, frameworks, commands)
		case "tsconfig.json":
			ecosystems["TypeScript"] = true
			addManifest(path, "TypeScript configuration", "TypeScript")
		case "pyproject.toml", "setup.py", "setup.cfg", "pipfile", "requirements.txt":
			ecosystems["Python"] = true
			addManifest(path, "Python project", "Python")
		case "pom.xml":
			ecosystems["Java"] = true
			addManifest(path, "Maven project", "Java")
			commands["mvn test"] = true
			commands["mvn package"] = true
		case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts":
			ecosystems["JVM"] = true
			addManifest(path, "Gradle project", "JVM")
			if strings.HasSuffix(name, ".kts") {
				ecosystems["Kotlin"] = true
			}
			commands["gradle test"] = true
		case "cargo.toml":
			ecosystems["Rust"] = true
			addManifest(path, "Cargo project", "Rust")
			commands["cargo test"] = true
			commands["cargo build"] = true
		case "dockerfile":
			ecosystems["Docker"] = true
			addManifest(path, "Container build", "Docker")
		case "cmakelists.txt":
			ecosystems["C/C++"] = true
			addManifest(path, "CMake project", "C/C++")
			commands["cmake --build build"] = true
		case "makefile", "gnumakefile":
			ecosystems["C/C++"] = true
			addManifest(path, "Make project", "C/C++")
			commands["make test"] = true
		case "package.swift":
			ecosystems["Swift"] = true
			addManifest(path, "Swift package", "Swift")
			commands["swift test"] = true
		case "pubspec.yaml":
			ecosystems["Dart"] = true
			addManifest(path, "Dart/Flutter project", "Dart")
			commands["dart test"] = true
		case "gemfile":
			ecosystems["Ruby"] = true
			addManifest(path, "Ruby project", "Ruby")
		case "composer.json":
			ecosystems["PHP"] = true
			addManifest(path, "PHP Composer project", "PHP")
		case "mix.exs":
			ecosystems["Elixir"] = true
			addManifest(path, "Elixir Mix project", "Elixir")
			commands["mix test"] = true
		}
		if strings.HasSuffix(name, ".csproj") || strings.HasSuffix(name, ".sln") || strings.HasSuffix(name, ".fsproj") {
			ecosystems[".NET"] = true
			addManifest(path, ".NET project", ".NET")
			commands["dotnet test"] = true
			commands["dotnet build"] = true
		}
		switch filepath.Ext(name) {
		case ".tf":
			ecosystems["Terraform"] = true
			commands["terraform validate"] = true
		case ".sql":
			ecosystems["SQL"] = true
		case ".graphql", ".gql":
			ecosystems["GraphQL"] = true
		case ".proto":
			ecosystems["Protocol Buffers"] = true
		}
		if strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.HasSuffix(name, "_test.go") {
			addTestSignal(filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil {
		return ActionResult{Success: false, Error: fmt.Sprintf("scan repository: %v", walkErr), Diagnostics: report}
	}
	if ecosystems["Python"] && len(testSignals) > 0 {
		commands["python -m pytest"] = true
	}
	if ecosystems["Ruby"] && len(testSignals) > 0 {
		commands["bundle exec rake test"] = true
	}
	if ecosystems["Dart"] && len(testSignals) > 0 {
		commands["dart test"] = true
	}
	for value := range ecosystems {
		report.Ecosystems = append(report.Ecosystems, value)
	}
	for value := range frameworks {
		report.Frameworks = append(report.Frameworks, value)
	}
	for value := range commands {
		report.ValidationCommands = append(report.ValidationCommands, value)
	}
	for value := range testSignals {
		report.TestSignals = append(report.TestSignals, value)
	}
	for value := range moduleRoots {
		report.Modules = append(report.Modules, value)
	}
	for value := range ignored {
		report.IgnoredGenerated = append(report.IgnoredGenerated, value)
	}
	sort.Strings(report.Ecosystems)
	sort.Strings(report.Frameworks)
	sort.Strings(report.ValidationCommands)
	sort.Strings(report.TestSignals)
	sort.Strings(report.Modules)
	sort.Strings(report.IgnoredGenerated)
	sort.Slice(report.Manifests, func(i, j int) bool { return report.Manifests[i].Path < report.Manifests[j].Path })
	report.ProjectType = projectType(report.Ecosystems, report.Languages)
	if report.FilesScanned >= params.MaxFiles {
		report.Warnings = append(report.Warnings, fmt.Sprintf("file scan reached the %d-file orientation budget; narrow path or increase max_files deliberately", params.MaxFiles))
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Detected %s stack from %d files and %d manifest(s)", report.ProjectType, report.FilesScanned, len(report.Manifests)), Diagnostics: report, FilesRead: []string{root}}
}

func inspectPackageManifest(path string, ecosystems, frameworks, commands map[string]bool) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) > maxStackMarkerBytes {
		return
	}
	var manifest struct {
		Scripts      map[string]string          `json:"scripts"`
		Dependencies map[string]json.RawMessage `json:"dependencies"`
		Dev          map[string]json.RawMessage `json:"devDependencies"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return
	}
	packages := map[string]bool{}
	for name := range manifest.Dependencies {
		packages[strings.ToLower(name)] = true
	}
	for name := range manifest.Dev {
		packages[strings.ToLower(name)] = true
	}
	for packageName, framework := range map[string]string{"react": "React", "next": "Next.js", "vue": "Vue", "svelte": "Svelte", "@angular/core": "Angular", "express": "Express", "@nestjs/core": "NestJS", "fastify": "Fastify", "vite": "Vite", "webpack": "Webpack", "electron": "Electron"} {
		if packages[packageName] {
			frameworks[framework] = true
		}
	}
	if packages["typescript"] {
		ecosystems["TypeScript"] = true
	}
	packageManager := "npm"
	directory := filepath.Dir(path)
	if _, err := os.Stat(filepath.Join(directory, "pnpm-lock.yaml")); err == nil {
		packageManager = "pnpm"
	} else if _, err := os.Stat(filepath.Join(directory, "yarn.lock")); err == nil {
		packageManager = "yarn"
	} else if _, err := os.Stat(filepath.Join(directory, "bun.lockb")); err == nil {
		packageManager = "bun"
	}
	for script := range manifest.Scripts {
		if script == "test" || script == "build" || script == "lint" {
			commands[packageManager+" run "+script] = true
		}
	}
}

func isTestPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") || strings.Contains(lower, "/spec/") || strings.Contains(lower, "/__tests__/") || strings.HasPrefix(lower, "test/") || strings.HasPrefix(lower, "tests/")
}

func projectType(ecosystems []string, languages map[string]int) string {
	if len(ecosystems) > 0 {
		if len(ecosystems) == 1 {
			return ecosystems[0]
		}
		return "polyglot/multi-stack"
	}
	if len(languages) > 0 {
		return "source repository"
	}
	return "unknown"
}
