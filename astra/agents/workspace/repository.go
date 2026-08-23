package workspace

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type SearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}
type FileInspection struct {
	Package           string   `json:"package,omitempty"`
	Imports           []string `json:"imports,omitempty"`
	ExportedFunctions []string `json:"exported_functions,omitempty"`
	Structs           []string `json:"structs,omitempty"`
	Interfaces        []string `json:"interfaces,omitempty"`
}

// SearchText performs a bounded, line-oriented repository search without shelling out.
func (w *Workspace) SearchText(query string, limit int) ([]SearchMatch, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	matches := []SearchMatch{}
	err := filepath.WalkDir(w.Root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if ShouldSkipGeneratedDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ShouldSkipGeneratedFile(entry.Name()) {
			return nil
		}
		if len(matches) >= limit {
			return filepath.SkipAll
		}
		info, err := entry.Info()
		if err != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
				rel, _ := filepath.Rel(w.Root, path)
				matches = append(matches, SearchMatch{File: filepath.ToSlash(rel), Line: line, Snippet: strings.TrimSpace(text)})
				if len(matches) >= limit {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return matches, err
}

func (w *Workspace) InspectGoFile(path string) (FileInspection, error) {
	abs, err := w.abs(path)
	if err != nil {
		return FileInspection{}, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, abs, nil, 0)
	if err != nil {
		return FileInspection{}, err
	}
	result := FileInspection{Package: file.Name.Name}
	for _, imp := range file.Imports {
		result.Imports = append(result.Imports, strings.Trim(imp.Path.Value, "\""))
	}
	for _, declaration := range file.Decls {
		switch d := declaration.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				result.ExportedFunctions = append(result.ExportedFunctions, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || !typeSpec.Name.IsExported() {
					continue
				}
				switch typeSpec.Type.(type) {
				case *ast.StructType:
					result.Structs = append(result.Structs, typeSpec.Name.Name)
				case *ast.InterfaceType:
					result.Interfaces = append(result.Interfaces, typeSpec.Name.Name)
				}
			}
		}
	}
	return result, nil
}
