package workspace

import "strings"

// ShouldSkipGeneratedDirectory identifies dependency trees, build output, and
// generated caches that should not consume repository orientation budgets.
func ShouldSkipGeneratedDirectory(name string) bool {
	switch name {
	case ".git", ".astra", ".cache", ".mypy_cache", ".pytest_cache", ".tox", ".venv", "__pycache__", "build", "coverage", "dist", "node_modules", "target", "vendor", "venv":
		return true
	default:
		return false
	}
}

// ShouldSkipGeneratedFile identifies binary and generated files that are not
// useful to text-oriented repository search or structural analysis.
func ShouldSkipGeneratedFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".pyc", ".pyo", ".class", ".o", ".obj", ".so", ".dylib", ".dll", ".exe", ".zip", ".gz", ".tar", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
