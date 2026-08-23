package main

import (
	"astra/astra/agents/core"
	"astra/astra/sources/promptstore"
	"astra/astra/sources/scope"
	"astra/astra/sources/state"
	colorutil "astra/astra/utils/color"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// printDashboard is intentionally read-only. It combines local workspace,
// session, Mind Palace, and sync facts without inventing metrics that are not
// persisted by the runtime.
func printDashboard(root, memoryRoot, provider, model string, agent *core.BaseAgent, active int) {
	workspaceFiles, workspaceDirs := countFiles(root)
	artifactFiles, _ := countFiles(state.SessionArtifactsRoot(root, agent.SessionID))
	attachmentFiles, _ := countFiles(state.SessionAttachmentsRoot(root, agent.SessionID))
	memoryFiles, memoryDirs := countFiles(filepath.Join(memoryRoot, "users", fmt.Sprintf("%d", agent.UserID), "memory"))
	sessionEvents := filepath.Join(memoryRoot, "users", fmt.Sprintf("%d", agent.UserID), "sessions", safeViewName(agent.SessionID), "events.jsonl")

	printViewHeader("Astra dashboard", root)
	printKV("Project", filepath.Base(root))
	printKV("Session", agent.SessionID)
	printKV("Model", provider+"/"+model)
	printKV("Workspace", fmt.Sprintf("%d files, %d directories", workspaceFiles, workspaceDirs))
	printKV("Artifacts", fmt.Sprintf("%d managed files", artifactFiles))
	printKV("Attachments", fmt.Sprintf("%d files", attachmentFiles))
	printKV("Mind Palace", fmt.Sprintf("%d memory files across %d kinds", memoryFiles, memoryDirs))
	printKV("Session evidence", fileState(sessionEvents))
	printKV("Requests", fmt.Sprintf("%d active", active))
	if scopes, err := scope.Default().List(); err == nil {
		printKV("Approved scopes", fmt.Sprintf("%d directories", len(scopes)))
	}
	if profiles, err := promptstore.Default().List(); err == nil {
		printKV("Prompt profiles", fmt.Sprintf("%d configured", len(profiles)))
	}

	printSection("Activity guide")
	printBar("Workspace", workspaceFiles, maxInt(workspaceFiles, 1), colorutil.ColorInfo)
	printBar("Artifacts", artifactFiles, maxInt(workspaceFiles, 1), colorutil.ColorFinalSuccess)
	printBar("Memory", memoryFiles, maxInt(workspaceFiles, 1), colorutil.ColorPrompt)
	printSection("Views")
	printCLIText(colorutil.ColorInfo(":chat  :workspace  :mindpalace  :sessions  :sync  :help"))
}

func printWorkspaceView(root string) {
	printViewHeader("Project workspace", root)
	if err := listCLIFiles(root, root, true, 0); err != nil {
		printCLIText(colorutil.ColorError(err.Error()))
	}
}

func printMindPalaceView(memoryRoot string, userID int) {
	path := filepath.Join(memoryRoot, "users", fmt.Sprintf("%d", userID), "memory")
	printViewHeader("User Mind Palace", path)
	files, dirs := countFiles(path)
	printKV("Memory blocks", fmt.Sprintf("%d files", files))
	printKV("Knowledge kinds", fmt.Sprintf("%d directories", dirs))
	if _, err := os.Stat(path); err != nil {
		printCLIText(colorutil.ColorInfo("No durable memory has been created yet."))
		return
	}
	if err := listCLIFiles(path, path, true, 0); err != nil {
		printCLIText(colorutil.ColorError(err.Error()))
	}
}

func printSessionsView(root, memoryRoot string, userID int) {
	printViewHeader("Project sessions", state.ProjectDataRoot(root))
	projectSessions := filepath.Join(state.ProjectDataRoot(root), "sessions")
	if entries, err := os.ReadDir(projectSessions); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				fmt.Printf("  local %-24s %s\n", entry.Name(), fileState(filepath.Join(projectSessions, entry.Name(), "manifest.json")))
			}
		}
	} else if !os.IsNotExist(err) {
		printCLIText(colorutil.ColorError(err.Error()))
	}

	path := filepath.Join(memoryRoot, "users", fmt.Sprintf("%d", userID), "sessions")
	printSection("Durable session evidence")
	printKV("Root", path)
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		printCLIText(colorutil.ColorInfo("No session evidence has been recorded yet."))
		return
	}
	if err != nil {
		printCLIText(colorutil.ColorError(err.Error()))
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			_, size := sessionEventInfo(filepath.Join(path, entry.Name(), "events.jsonl"))
			fmt.Printf("  %s  %d events\n", entry.Name(), size)
		}
	}
}

func printSyncView(root, memoryRoot string) {
	printViewHeader("Astra sync", root)
	printKV("Mind Palace root", memoryRoot)
	printKV("Mode", "local files only")
	printKV("Managed scope", "Mind Palace, session evidence, and artifacts")
	syncFiles, _ := countFiles(filepath.Join(state.ProjectDataRoot(root), "sessions"))
	printKV("Sync records", fmt.Sprintf("%d local records", syncFiles))
	printKV("Source repository", "local; never exported implicitly")
	printCLIText(colorutil.ColorInfo("External sync is disabled. Files are already durable on this machine."))
}

func printScopesView() {
	printViewHeader("Approved filesystem scopes", scope.Default().Path())
	entries, err := scope.Default().List()
	if err != nil {
		printCLIText(colorutil.ColorError(err.Error()))
		return
	}
	if len(entries) == 0 {
		printCLIText(colorutil.ColorInfo("No approved scopes. Use :scope add <directory> all."))
		return
	}
	for _, entry := range entries {
		label := entry.Label
		if label == "" {
			label = "unlabelled"
		}
		fmt.Printf("  %-18s %-20s %s  [%s]\n", entry.ID, label, entry.Path, strings.Join(entry.Permissions, ", "))
	}
}

func printPromptProfilesView() {
	printViewHeader("Global prompt profiles", promptstore.Default().Root())
	profiles, err := promptstore.Default().List()
	if err != nil {
		printCLIText(colorutil.ColorError(err.Error()))
		return
	}
	if len(profiles) == 0 {
		printCLIText(colorutil.ColorInfo("No prompt profiles configured."))
		return
	}
	for _, profile := range profiles {
		status := "disabled"
		if profile.Enabled {
			status = "enabled"
		}
		fmt.Printf("  %-24s %-8s %s\n", profile.Name, status, profile.File)
	}
}

func printAgentsView(agent *core.BaseAgent) {
	printViewHeader("Agent branches", "current Astra supervisor")
	branches := agent.ListAgents()
	if len(branches) == 0 {
		printCLIText(colorutil.ColorInfo("No worker branches have been spawned."))
		return
	}
	for _, branch := range branches {
		fmt.Printf("  %-32s %-10s %-16s %s\n", branch.ID, branch.Status, branch.Name, branch.Goal)
		fmt.Printf("    scope: %s  model: %s/%s  events: %d\n", branch.WorkspaceRoot, branch.Provider, branch.Model, branch.Events)
		if branch.Error != "" {
			fmt.Println(colorutil.ColorError("    error: " + branch.Error))
		}
	}
}

func scopeViewText() string {
	entries, err := scope.Default().List()
	if err != nil {
		return "Could not read approved scopes: " + err.Error()
	}
	if len(entries) == 0 {
		return "No approved scopes. Use: astra scope add <directory> all"
	}
	var builder strings.Builder
	builder.WriteString("Approved filesystem scopes\n")
	for _, entry := range entries {
		fmt.Fprintf(&builder, "• %s [%s] %s\n", entry.ID, strings.Join(entry.Permissions, ","), entry.Path)
	}
	return strings.TrimSpace(builder.String())
}

func countScopes() int {
	entries, err := scope.Default().List()
	if err != nil {
		return 0
	}
	return len(entries)
}

func countPromptProfiles() int {
	profiles, err := promptstore.Default().List()
	if err != nil {
		return 0
	}
	return len(profiles)
}

func promptProfilesViewText() string {
	profiles, err := promptstore.Default().List()
	if err != nil {
		return "Could not read prompt profiles: " + err.Error()
	}
	if len(profiles) == 0 {
		return "No prompt profiles configured. Profiles live under " + promptstore.Default().Root()
	}
	var builder strings.Builder
	builder.WriteString("Global prompt profiles\n")
	for _, profile := range profiles {
		status := "disabled"
		if profile.Enabled {
			status = "enabled"
		}
		fmt.Fprintf(&builder, "• %s [%s] %s\n", profile.Name, status, profile.File)
	}
	return strings.TrimSpace(builder.String())
}

func printViewHeader(title, path string) {
	printCLIText(colorutil.ColorPrompt("\n" + title))
	printCLIText(colorutil.ColorInfo("Path: " + path))
}

func printSection(title string) { printCLIText(colorutil.ColorPrompt("\n" + title)) }

func printKV(key, value string) { fmt.Printf("  %-18s %s\n", key, value) }

func printBar(label string, value, total int, color func(string) string) {
	if total <= 0 {
		total = 1
	}
	width := 28
	filled := value * width / total
	if filled > width {
		filled = width
	}
	if value > 0 && filled == 0 {
		filled = 1
	}
	fmt.Printf("  %-12s %s %d\n", label, color(strings.Repeat("█", filled)+strings.Repeat("·", width-filled)), value)
}

func countFiles(root string) (files, dirs int) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == ".astra" {
			continue
		}
		if entry.IsDir() {
			dirs++
			nestedFiles, nestedDirs := countFiles(filepath.Join(root, entry.Name()))
			files += nestedFiles
			dirs += nestedDirs
		} else {
			files++
		}
	}
	return files, dirs
}

func sessionEventInfo(path string) (time.Time, int) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return info.ModTime(), 0
	}
	return info.ModTime(), maxInt(0, len(strings.Split(strings.TrimSpace(string(data)), "\n")))
}

func fileState(path string) string {
	if info, err := os.Stat(path); err == nil {
		return fmt.Sprintf("present (%d bytes, updated %s)", info.Size(), info.ModTime().Format(time.RFC3339))
	}
	return "not created"
}

func safeViewName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.NewReplacer("/", "-", "\\", "-", "..", "-").Replace(value)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
