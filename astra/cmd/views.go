package main

import (
	"astra/astra/agents/core"
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
	artifactFiles, _ := countFiles(filepath.Join(root, ".astra", "artifacts"))
	attachmentFiles, _ := countFiles(filepath.Join(root, ".astra", "attachments"))
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
	printViewHeader("Project sessions", filepath.Join(root, ".astra", "sessions"))
	projectSessions := filepath.Join(root, ".astra", "sessions")
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
	syncFiles, _ := countFiles(filepath.Join(root, ".astra", "sync"))
	printKV("Sync records", fmt.Sprintf("%d local records", syncFiles))
	printKV("Source repository", "local; never exported implicitly")
	printCLIText(colorutil.ColorInfo("External sync is disabled. Files are already durable on this machine."))
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
		if entry.Name() == ".git" || entry.Name() == "node_modules" {
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
