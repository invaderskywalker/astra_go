package actions

import (
	"astra/astra/agents/workspace"
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// AnalyzeFilesParams requests compact structural evidence without returning
// source bodies. Paths may be files or directories inside the connected root.
// A later read_files call can use the returned recommended ranges.
type AnalyzeFilesParams struct {
	Paths     []string `json:"paths"`
	Query     string   `json:"query,omitempty"`
	Recursive bool     `json:"recursive,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type FileRange struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

type FileMatch struct {
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

// FileAnalysis is metadata-first. It is safe to pass through a planning
// prompt even for very large repositories because it never contains a body.
type FileAnalysis struct {
	Path              string      `json:"path"`
	Language          string      `json:"language,omitempty"`
	Bytes             int64       `json:"bytes"`
	Lines             int         `json:"lines"`
	ModifiedAt        time.Time   `json:"modified_at"`
	SHA256            string      `json:"sha256,omitempty"`
	Headings          []string    `json:"headings,omitempty"`
	Symbols           []string    `json:"symbols,omitempty"`
	Imports           []string    `json:"imports,omitempty"`
	Matches           []FileMatch `json:"matches,omitempty"`
	RecommendedRanges []FileRange `json:"recommended_ranges,omitempty"`
	Warnings          []string    `json:"warnings,omitempty"`
}

var (
	goSymbolRE   = regexp.MustCompile(`^(?:func|type|var|const)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	genericSymRE = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?(?:function|class|interface|type|def|fn)\s+([A-Za-z_][A-Za-z0-9_]*)`)
)

func (a *DataActions) AnalyzeFiles(params AnalyzeFilesParams) ActionResult {
	paths := params.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if params.Limit <= 0 || params.Limit > 128 {
		params.Limit = 64
	}
	files := make([]string, 0, params.Limit)
	for _, requested := range paths {
		abs, err := a.workspace.ResolvePath(requested)
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("analyze %s: %v", requested, err)}
		}
		info, err := os.Stat(abs)
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("stat %s: %v", requested, err)}
		}
		if !info.IsDir() {
			files = append(files, requested)
			continue
		}
		walkErr := filepath.WalkDir(abs, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if workspace.ShouldSkipGeneratedDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				if !params.Recursive && path != abs {
					return filepath.SkipDir
				}
				return nil
			}
			if len(files) >= params.Limit {
				return filepath.SkipAll
			}
			if workspace.ShouldSkipGeneratedFile(entry.Name()) {
				return nil
			}
			rel, _ := filepath.Rel(a.workspace.Root, path)
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if walkErr != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("walk %s: %v", requested, walkErr)}
		}
		if len(files) >= params.Limit {
			break
		}
	}
	if len(files) > params.Limit {
		files = files[:params.Limit]
	}
	profiles := make([]FileAnalysis, 0, len(files))
	for _, path := range files {
		profile, err := analyzeFile(a.workspace.Root, path, params.Query)
		if err != nil {
			return ActionResult{Success: false, Error: fmt.Sprintf("analyze %s: %v", path, err), Diagnostics: profiles}
		}
		profiles = append(profiles, profile)
	}
	return ActionResult{Success: true, Summary: fmt.Sprintf("Analyzed %d file(s) without reading full bodies", len(profiles)), Diagnostics: profiles, FilesRead: files}
}

func analyzeFile(root, relative, query string) (FileAnalysis, error) {
	abs := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Stat(abs)
	if err != nil {
		return FileAnalysis{}, err
	}
	profile := FileAnalysis{Path: filepath.ToSlash(relative), Language: languageForPath(relative), Bytes: info.Size(), ModifiedAt: info.ModTime()}
	file, err := os.Open(abs)
	if err != nil {
		return profile, err
	}
	defer file.Close()
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(file, hash))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	needle := strings.ToLower(strings.TrimSpace(query))
	for scanner.Scan() {
		profile.Lines++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") && len(profile.Headings) < 30 {
			profile.Headings = append(profile.Headings, strings.TrimSpace(strings.TrimLeft(line, "#")))
		}
		if symbol := symbolName(line, profile.Language); symbol != "" && len(profile.Symbols) < 80 {
			profile.Symbols = append(profile.Symbols, symbol)
		}
		if imp := importName(line, profile.Language); imp != "" && len(profile.Imports) < 80 {
			profile.Imports = append(profile.Imports, imp)
		}
		if needle != "" && strings.Contains(strings.ToLower(line), needle) && len(profile.Matches) < 24 {
			profile.Matches = append(profile.Matches, FileMatch{Line: profile.Lines, Snippet: clipAnalysisText(line)})
		}
	}
	if err := scanner.Err(); err != nil {
		profile.Warnings = append(profile.Warnings, "line scan stopped: "+err.Error())
	}
	profile.SHA256 = fmt.Sprintf("%x", hash.Sum(nil))
	for _, match := range profile.Matches {
		start := match.Line - 3
		if start < 1 {
			start = 1
		}
		end := match.Line + 3
		if end > profile.Lines {
			end = profile.Lines
		}
		profile.RecommendedRanges = appendUniqueRange(profile.RecommendedRanges, FileRange{StartLine: start, EndLine: end})
		if len(profile.RecommendedRanges) >= 12 {
			break
		}
	}
	if len(profile.RecommendedRanges) == 0 && profile.Lines > 0 {
		end := profile.Lines
		if end > 80 {
			end = 80
		}
		profile.RecommendedRanges = []FileRange{{StartLine: 1, EndLine: end}}
	}
	return profile, nil
}

func appendUniqueRange(ranges []FileRange, candidate FileRange) []FileRange {
	for _, existing := range ranges {
		if existing == candidate {
			return ranges
		}
	}
	return append(ranges, candidate)
}

func languageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".md", ".mdx":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".css", ".scss":
		return "css"
	case ".html":
		return "html"
	default:
		return "text"
	}
}

func symbolName(line, language string) string {
	if language == "go" {
		match := goSymbolRE.FindStringSubmatch(line)
		if len(match) == 2 {
			return match[1]
		}
	}
	match := genericSymRE.FindStringSubmatch(line)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func importName(line, language string) string {
	if language == "go" && strings.HasPrefix(line, "import ") {
		return strings.TrimSpace(strings.Trim(line[7:], "()\""))
	}
	if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
		return clipAnalysisText(line)
	}
	return ""
}

func clipAnalysisText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= 800 {
		return text
	}
	return text[:800] + fmt.Sprintf("… (%d chars)", len(text))
}
