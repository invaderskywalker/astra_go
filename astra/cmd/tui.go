package main

// tui.go contains Astra's interactive cockpit.  The plain CLI remains useful
// for pipes and automation, but a connected terminal deserves an application
// shell: output never competes with the editor, navigation is explicit, and
// every view is built from the same persisted workspace facts.

import (
	"astra/astra/agents/core"
	"astra/astra/sources/state"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiTab int

const (
	tabChat tuiTab = iota
	tabDashboard
	tabWorkspace
	tabMindPalace
	tabSessions
	tabSync
)

var tuiTabs = []struct {
	name string
	icon string
}{
	{"Chat", "◈"},
	{"Dashboard", "◫"},
	{"Workspace", "⌂"},
	{"Mind Palace", "✦"},
	{"Sessions", "◷"},
	{"Sync", "⇄"},
}

type tuiEventMsg struct {
	id      int
	message string
	done    bool
}

type tuiEntry struct {
	kind string
	text string
}

type tuiModel struct {
	agent      *core.BaseAgent
	root       string
	memoryRoot string
	provider   string
	model      string

	tab    tuiTab
	width  int
	height int

	input    textarea.Model
	viewport viewport.Model
	entries  []tuiEntry
	streams  map[int]string
	active   map[int]bool
	outputs  map[int]<-chan string
	nextID   int
	status   string
	quitting bool
}

func runAstraTUI(agent *core.BaseAgent, root, memoryRoot, provider, model string) error {
	m := newTUIModel(agent, root, memoryRoot, provider, model)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func newTUIModel(agent *core.BaseAgent, root, memoryRoot, provider, model string) *tuiModel {
	ta := textarea.New()
	ta.Placeholder = "Ask Astra to inspect, build, explain, test, or write…"
	ta.Prompt = "  › "
	ta.CharLimit = 12000
	ta.SetHeight(3)
	ta.KeyMap.InsertNewline.SetKeys("ctrl+j")
	ta.Focus()

	vp := viewport.New(80, 20)
	m := &tuiModel{
		agent: agent, root: root, memoryRoot: memoryRoot, provider: provider, model: model,
		tab: tabChat, input: ta, viewport: vp, streams: map[int]string{}, active: map[int]bool{}, nextID: 1,
		status: "Ready", width: 100, height: 30, outputs: map[int]<-chan string{},
	}
	m.refreshViewport()
	return m
}

func (m *tuiModel) Init() tea.Cmd { return textarea.Blink }

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		m.refreshViewport()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tuiEventMsg:
		if msg.done {
			delete(m.active, msg.id)
			delete(m.outputs, msg.id)
			if text := strings.TrimSpace(m.streams[msg.id]); text != "" {
				m.entries = append(m.entries, tuiEntry{kind: "assistant", text: text})
			}
			delete(m.streams, msg.id)
			m.status = fmt.Sprintf("Request #%d completed", msg.id)
		} else {
			m.consumeEvent(msg.id, msg.message)
		}
		m.refreshViewport()
		if output, ok := m.outputs[msg.id]; ok {
			return m, readTUIEvent(output, msg.id)
		}
		return m, nil
	case tea.MouseMsg:
		if msg.Type == tea.MouseWheelUp || msg.Type == tea.MouseWheelDown {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	if m.tab == tabChat {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		if strings.TrimSpace(m.input.Value()) != "" {
			m.input.Reset()
			m.status = "Draft cleared"
			return m, nil
		}
		if len(m.active) > 0 {
			m.agent.Stop()
		}
		return m, tea.Quit
	case "ctrl+q", "q":
		if m.tab != tabChat || strings.TrimSpace(m.input.Value()) == "" {
			if len(m.active) > 0 {
				m.agent.Stop()
			}
			return m, tea.Quit
		}
	case "ctrl+p":
		m.agent.Pause()
		m.status = "Pause requested; Astra will stop at a safe checkpoint"
		return m, nil
	case "ctrl+r":
		m.agent.Resume()
		m.status = "Astra resumed"
		return m, nil
	case "ctrl+x":
		if m.agent.Stop() {
			m.status = "Stop requested; active work will cancel safely"
		} else {
			m.status = "No active request"
		}
		return m, nil
	case "ctrl+l":
		m.entries = nil
		m.streams = map[int]string{}
		m.refreshViewport()
		m.status = "Chat transcript cleared"
		return m, nil
	case "enter":
		if m.tab == tabChat {
			return m, m.submit()
		}
	case "tab", "right":
		m.tab = tuiTab((int(m.tab) + 1) % len(tuiTabs))
		m.status = tuiTabs[m.tab].name
		m.refreshViewport()
		return m, nil
	case "shift+tab", "left":
		m.tab = tuiTab((int(m.tab) + len(tuiTabs) - 1) % len(tuiTabs))
		m.status = tuiTabs[m.tab].name
		m.refreshViewport()
		return m, nil
	case "up":
		if m.tab != tabChat {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case "down":
		if m.tab != tabChat {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case "pgup", "pgdown", "home", "end":
		if m.tab != tabChat {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5", "ctrl+6":
		m.tab = tuiTab(int(key[len(key)-1] - '1'))
		m.status = tuiTabs[m.tab].name
		m.refreshViewport()
		return m, nil
	}

	if m.tab == tabChat {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tuiModel) submit() tea.Cmd {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return nil
	}
	if strings.HasPrefix(query, ":") {
		m.entries = append(m.entries, tuiEntry{kind: "user", text: query})
		m.input.Reset()
		m.handleLocalCommand(query)
		m.refreshViewport()
		if m.quitting {
			if len(m.active) > 0 {
				m.agent.Stop()
			}
			return tea.Quit
		}
		return nil
	}
	if len(query) > 12000 {
		query = query[:12000]
	}
	id := m.nextID
	m.nextID++
	m.active[id] = true
	m.entries = append(m.entries, tuiEntry{kind: "user", text: query})
	m.input.Reset()
	m.status = fmt.Sprintf("Working on request #%d…", id)
	m.refreshViewport()
	output := m.agent.ProcessQuery(query)
	m.outputs[id] = output
	return readTUIEvent(output, id)
}

func (m *tuiModel) handleLocalCommand(query string) {
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return
	}
	command := strings.TrimPrefix(parts[0], ":")
	switch command {
	case "chat":
		m.tab = tabChat
		m.status = "Chat"
	case "dashboard":
		m.tab = tabDashboard
		m.status = "Dashboard"
	case "workspace", "tree", "ls":
		m.tab = tabWorkspace
		m.status = "Workspace"
	case "mindpalace", "memory":
		m.tab = tabMindPalace
		m.status = "Mind Palace"
	case "sessions", "session":
		m.tab = tabSessions
		m.status = "Sessions"
	case "sync":
		m.tab = tabSync
		m.status = "Sync"
	case "model":
		if len(parts) != 3 {
			m.entries = append(m.entries, tuiEntry{kind: "question", text: "Usage: :model <ollama|openai> <model>"})
			return
		}
		if len(m.active) > 0 {
			m.entries = append(m.entries, tuiEntry{kind: "question", text: "Wait for active requests to finish before switching models."})
			return
		}
		if err := m.agent.SetModel(parts[1], parts[2]); err != nil {
			m.entries = append(m.entries, tuiEntry{kind: "error", text: err.Error()})
			return
		}
		m.provider, m.model = parts[1], parts[2]
		m.status = "Switched to " + parts[1] + "/" + parts[2]
		m.entries = append(m.entries, tuiEntry{kind: "status", text: m.status + " for future requests."})
	case "pause":
		m.agent.Pause()
		m.status = "Pause requested"
	case "resume":
		m.agent.Resume()
		m.status = "Astra resumed"
	case "stop", "cancel":
		if m.agent.Stop() {
			m.status = "Stop requested"
		} else {
			m.status = "No active request"
		}
	case "clear", "abort":
		stopped := m.agent.Stop()
		cleared := m.agent.ClearPending()
		m.status = fmt.Sprintf("Cleared %d queued request(s)", cleared)
		if stopped {
			m.status += "; active cancellation requested"
		}
	case "pwd":
		m.entries = append(m.entries, tuiEntry{kind: "info", text: m.root})
	case "attach":
		if len(parts) != 2 {
			m.entries = append(m.entries, tuiEntry{kind: "question", text: "Usage: :attach <file-path>"})
			return
		}
		if target, err := attachCLIFile(m.root, m.agent.SessionID, parts[1]); err != nil {
			m.entries = append(m.entries, tuiEntry{kind: "error", text: err.Error()})
		} else {
			m.entries = append(m.entries, tuiEntry{kind: "status", text: "Attached: " + target})
		}
	case "help":
		m.entries = append(m.entries, tuiEntry{kind: "info", text: ":dashboard :workspace :mindpalace :sessions :sync :model :pause :resume :stop :clear :attach :pwd :quit"})
	case "quit", "exit":
		m.quitting = true
		return
	default:
		m.entries = append(m.entries, tuiEntry{kind: "question", text: "Unknown local command. Use :help or type a normal request."})
	}
	if m.quitting {
		return
	}
}

func readTUIEvent(output <-chan string, id int) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-output
		if !ok {
			return tuiEventMsg{id: id, done: true}
		}
		return tuiEventMsg{id: id, message: message}
	}
}

func (m *tuiModel) consumeEvent(id int, raw string) {
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		m.entries = append(m.entries, tuiEntry{kind: "info", text: raw})
		return
	}
	typ, _ := event["type"].(string)
	payload, _ := event["payload"].(map[string]interface{})
	switch typ {
	case "response_chunk":
		if chunk, ok := payload["chunk"].(string); ok {
			m.streams[id] += chunk
		}
	case "status", "intermediate":
		if text := tuiString(payload, "message"); text != "" {
			m.status = text
			m.entries = append(m.entries, tuiEntry{kind: "status", text: text})
		}
	case "plan":
		m.entries = append(m.entries, tuiEntry{kind: "plan", text: formatTUIPlan(payload)})
		m.status = "Plan ready"
	case "action_activation":
		if names := tuiStrings(payload, "actions"); len(names) > 0 {
			m.entries = append(m.entries, tuiEntry{kind: "tool", text: "Loaded tool documentation: " + strings.Join(names, ", ")})
		}
	case "action_plan":
		if step, ok := payload["step"].(map[string]interface{}); ok {
			m.entries = append(m.entries, tuiEntry{kind: "tool", text: formatTUIStep(step)})
		}
	case "action_result":
		m.entries = append(m.entries, tuiEntry{kind: "tool", text: formatTUIActionResult(payload)})
	case "needs_input":
		text := tuiString(payload, "message")
		if questions := tuiStrings(payload, "questions"); len(questions) > 0 {
			text += "\n" + strings.Join(questions, "\n")
		}
		m.entries = append(m.entries, tuiEntry{kind: "question", text: strings.TrimSpace(text)})
	case "error":
		m.entries = append(m.entries, tuiEntry{kind: "error", text: tuiString(payload, "message")})
	case "paused", "stopped", "completed":
		m.entries = append(m.entries, tuiEntry{kind: typ, text: tuiString(payload, "message")})
	}
}

func (m *tuiModel) resize() {
	mainWidth := maxTUI(40, m.width-27)
	mainHeight := maxTUI(8, m.height-11)
	contentWidth := maxTUI(24, mainWidth-6)
	contentHeight := maxTUI(4, mainHeight-4)
	m.viewport.Width, m.viewport.Height = contentWidth, contentHeight
	m.input.SetWidth(contentWidth)
	m.input.SetHeight(3)
}

func (m *tuiModel) refreshViewport() {
	if m.width < 40 {
		m.width = 40
	}
	if m.height < 16 {
		m.height = 16
	}
	m.resize()
	if m.tab == tabChat {
		var b strings.Builder
		for _, entry := range m.entries {
			b.WriteString(tuiEntryView(entry, m.viewport.Width))
			b.WriteString("\n")
		}
		for id, stream := range m.streams {
			if strings.TrimSpace(stream) != "" {
				b.WriteString(tuiStyleStreaming.Render(fmt.Sprintf("Request #%d\n%s", id, stream)))
				b.WriteString("\n")
			}
		}
		if len(m.active) == 0 && len(m.entries) == 0 {
			b.WriteString(tuiStyleMuted.Render("Start with a goal. Astra will inspect evidence before it acts.\n\nExamples\n  “Review this repository and tell me what is missing.”\n  “Create a technical design document in docs/.”\n  “Run the right tests and explain any failures.”"))
		}
		m.viewport.SetContent(strings.TrimRight(b.String(), "\n"))
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetContent(m.viewTab())
	m.viewport.GotoTop()
}

func (m *tuiModel) View() string {
	if m.width < 40 {
		return "Resize the terminal to at least 40 columns."
	}
	brand := tuiStyleBrand.Render("ASTRA") + tuiStyleMuted.Render("  /  personal engineering cockpit")
	status := tuiStyleMuted.Render(fmt.Sprintf("%s  ·  %s/%s  ·  %s", filepath.Base(m.root), m.provider, m.model, m.status))
	header := tuiStyleHeader.Width(m.width - 2).Render(brand + "\n" + status)

	sidebar := m.sidebar()
	mainWidth := maxTUI(40, m.width-27)
	mainHeight := maxTUI(8, m.height-11)
	main := tuiStyleMain.Width(mainWidth).Height(mainHeight).Render(m.viewport.View())
	content := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	footer := tuiStyleFooter.Width(m.width - 2).Render("Ctrl+J newline  ·  Enter send  ·  ↑↓/Tab navigate  ·  Ctrl+P pause  ·  Ctrl+R resume  ·  Ctrl+X stop  ·  Ctrl+L clear  ·  Ctrl+Q quit")
	if m.tab == tabChat {
		composerTitle := tuiStyleMuted.Render(fmt.Sprintf("MESSAGE  ·  %d active request(s)", len(m.active)))
		composer := tuiStyleComposer.Width(mainWidth).Render(composerTitle + "\n" + m.input.View())
		content = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, lipgloss.JoinVertical(lipgloss.Left, main, composer))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m *tuiModel) sidebar() string {
	rows := make([]string, 0, len(tuiTabs)+3)
	for i, tab := range tuiTabs {
		label := fmt.Sprintf("%d  %s  %s", i+1, tab.icon, tab.name)
		if tuiTab(i) == m.tab {
			rows = append(rows, tuiStyleSelected.Width(21).Render(label))
		} else {
			rows = append(rows, tuiStyleSidebar.Width(21).Render(label))
		}
	}
	rows = append(rows, "", tuiStyleMuted.Render("SESSION"), tuiStyleMuted.Render("  "+shortTUI(m.agent.SessionID, 20)), tuiStyleMuted.Render("  "+shortTUI(m.root, 20)))
	return tuiStyleSidebarBox.Height(maxTUI(8, m.height-11)).Render(strings.Join(rows, "\n"))
}

func (m *tuiModel) viewTab() string {
	switch m.tab {
	case tabDashboard:
		return m.dashboardView()
	case tabWorkspace:
		return m.workspaceView()
	case tabMindPalace:
		return m.mindPalaceView()
	case tabSessions:
		return m.sessionsView()
	case tabSync:
		return m.syncView()
	default:
		return ""
	}
}

func (m *tuiModel) dashboardView() string {
	workspaceFiles, workspaceDirs := countFiles(m.root)
	artifactFiles, _ := countFiles(state.SessionArtifactsRoot(m.root, m.agent.SessionID))
	attachmentFiles, _ := countFiles(state.SessionAttachmentsRoot(m.root, m.agent.SessionID))
	memoryFiles, memoryDirs := countFiles(filepath.Join(m.memoryRoot, "users", fmt.Sprintf("%d", m.agent.UserID), "memory"))
	lines := []string{
		tuiStyleTitle.Render("DASHBOARD"),
		"A calm view of what Astra can see, what it has written, and what is durable.",
		"",
		tuiMetric("Project", filepath.Base(m.root)),
		tuiMetric("Workspace", fmt.Sprintf("%d files  ·  %d directories", workspaceFiles, workspaceDirs)),
		tuiMetric("Artifacts", fmt.Sprintf("%d managed files", artifactFiles)),
		tuiMetric("Attachments", fmt.Sprintf("%d files", attachmentFiles)),
		tuiMetric("Mind Palace", fmt.Sprintf("%d memory files  ·  %d knowledge kinds", memoryFiles, memoryDirs)),
		tuiMetric("Requests", fmt.Sprintf("%d active", len(m.active))),
		"",
		tuiStyleSection.Render("ACTIVITY"),
		tuiBar("Workspace", workspaceFiles, maxTUI(workspaceFiles, 1), tuiColorCyan),
		tuiBar("Artifacts", artifactFiles, maxTUI(workspaceFiles, 1), tuiColorGreen),
		tuiBar("Memory", memoryFiles, maxTUI(workspaceFiles, 1), tuiColorMagenta),
		"",
		tuiStyleSection.Render("DESIGN PRINCIPLES"),
		"  • Evidence before conclusions",
		"  • User Mind Palace survives sessions",
		"  • Session workspace is visible and local-first",
		"  • Managed artifacts can sync; source code never uploads implicitly",
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) workspaceView() string {
	return m.fileView("WORKSPACE", m.root, "The connected project is visible here. Hidden build caches are omitted.")
}

func (m *tuiModel) mindPalaceView() string {
	path := filepath.Join(m.memoryRoot, "users", fmt.Sprintf("%d", m.agent.UserID), "memory")
	return m.fileView("MIND PALACE", path, "Durable user memory is organized as linked local files, not a learning database.")
}

func (m *tuiModel) fileView(title, root, description string) string {
	lines := []string{tuiStyleTitle.Render(title), description, "", tuiStyleMuted.Render(root), ""}
	files := collectTUITree(root)
	if len(files) == 0 {
		lines = append(lines, tuiStyleMuted.Render("No files yet."))
		return strings.Join(lines, "\n")
	}
	for _, file := range files {
		lines = append(lines, file)
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) sessionsView() string {
	project := filepath.Join(state.ProjectDataRoot(m.root), "sessions")
	lines := []string{tuiStyleTitle.Render("SESSIONS"), "Every session has a local manifest and durable evidence trail.", "", tuiStyleMuted.Render(project), ""}
	entries, _ := os.ReadDir(project)
	if len(entries) == 0 {
		return strings.Join(append(lines, tuiStyleMuted.Render("No session manifests yet.")), "\n")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lines = append(lines, "  ◷  "+entry.Name()+"  "+fileState(filepath.Join(project, entry.Name(), "manifest.json")))
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) syncView() string {
	syncFiles, _ := countFiles(filepath.Join(state.ProjectDataRoot(m.root), "sessions"))
	lines := []string{
		tuiStyleTitle.Render("SYNC & STORAGE"),
		"Local files are the source of truth. External sync is opt-in and currently disabled.",
		"",
		tuiMetric("Mind Palace root", m.memoryRoot),
		tuiMetric("Mode", "local files only"),
		tuiMetric("Sync records", fmt.Sprintf("%d local records", syncFiles)),
		"",
		tuiStyleSection.Render("POLICY"),
		"  ✓ Memory, artifacts, and sessions stay on this machine",
		"  ✓ Local status records explain every managed write",
		"  ! Nothing is uploaded implicitly",
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) consumeForTest(id int, raw string) { m.consumeEvent(id, raw) }

func tuiEntryView(entry tuiEntry, width int) string {
	text := strings.TrimSpace(entry.text)
	if text == "" {
		return ""
	}
	text = tuiWrap(text, maxTUI(20, width-8))
	switch entry.kind {
	case "user":
		return tuiStyleUser.Render("YOU") + "\n" + text
	case "assistant":
		return tuiStyleAssistant.Render("ASTRA") + "\n" + text
	case "plan":
		return tuiStylePlan.Render("PLAN") + "\n" + text
	case "tool":
		return tuiStyleTool.Render("TOOL") + "\n" + text
	case "question":
		return tuiStyleQuestion.Render("INPUT NEEDED") + "\n" + text
	case "error":
		return tuiStyleError.Render("ERROR") + "\n" + text
	case "paused", "stopped":
		return tuiStyleQuestion.Render(strings.ToUpper(entry.kind)) + "\n" + text
	default:
		return tuiStyleStatus.Render("STATUS") + "  " + text
	}
}

func formatTUIPlan(payload map[string]interface{}) string {
	parts := []string{}
	if mode := tuiString(payload, "mode"); mode != "" {
		parts = append(parts, "Mode: "+mode)
	}
	if goal := tuiString(payload, "goal"); goal != "" {
		parts = append(parts, "Goal: "+goal)
	}
	if skills := tuiStrings(payload, "selected_skills"); len(skills) > 0 {
		parts = append(parts, "Skills: "+strings.Join(skills, ", "))
	}
	if steps := tuiStrings(payload, "mind_map_steps_in_natural_language"); len(steps) > 0 {
		parts = append(parts, "Mind map:")
		for i, step := range steps {
			parts = append(parts, fmt.Sprintf("  %d. %s", i+1, step))
		}
	}
	if criteria := tuiStrings(payload, "success_criteria"); len(criteria) > 0 {
		parts = append(parts, "Success criteria: "+strings.Join(criteria, " · "))
	}
	return strings.Join(parts, "\n")
}

func formatTUIStep(step map[string]interface{}) string {
	parts := []string{}
	if action := tuiString(step, "action"); action != "" {
		parts = append(parts, action)
	}
	if reason := tuiString(step, "reason"); reason != "" {
		parts = append(parts, "Why: "+reason)
	}
	if expected := tuiString(step, "expected_observation"); expected != "" {
		parts = append(parts, "Expect: "+expected)
	}
	if params, ok := step["action_params"]; ok {
		if data, err := json.Marshal(params); err == nil && string(data) != "{}" {
			parts = append(parts, "Params: "+string(data))
		}
	}
	return strings.Join(parts, "\n")
}

func formatTUIActionResult(payload map[string]interface{}) string {
	action := tuiString(payload, "action")
	results, _ := payload["result"].(map[string]interface{})
	lines := []string{action}
	for _, raw := range results {
		entry, _ := raw.(map[string]interface{})
		if entry == nil {
			continue
		}
		prefix := "✓"
		if ok, _ := entry["success"].(bool); !ok {
			prefix = "✗"
		}
		message := tuiString(entry, "summary")
		if message == "" {
			message = tuiString(entry, "error")
		}
		lines = append(lines, fmt.Sprintf("%s %s", prefix, message))
		if commands, ok := entry["commands"].([]interface{}); ok {
			for _, rawCommand := range commands {
				command, _ := rawCommand.(map[string]interface{})
				if command == nil {
					continue
				}
				line := "  $ " + tuiString(command, "command")
				if errText := tuiString(command, "error"); errText != "" {
					line += "  ✗ " + errText
				}
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func collectTUIFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if path != root && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == ".astra") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, fmt.Sprintf("  %s  %s", rel, tuiStyleMuted.Render(humanTUISize(info.Size()))))
		return nil
	})
	sort.Strings(files)
	if len(files) > 250 {
		files = append(files[:250], tuiStyleMuted.Render("  … more files omitted; use :tree for a bounded listing"))
	}
	return files
}

func collectTUITree(root string) []string {
	var entries []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if path == root {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == ".astra" {
				return filepath.SkipDir
			}
			rel, _ := filepath.Rel(root, path)
			level := strings.Count(rel, string(os.PathSeparator)) + 1
			entries = append(entries, fmt.Sprintf("%s📁 %s/", strings.Repeat("  ", level), filepath.Base(path)))
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		level := strings.Count(rel, string(os.PathSeparator))
		entries = append(entries, fmt.Sprintf("%s📄 %s  %s", strings.Repeat("  ", level), filepath.Base(path), tuiStyleMuted.Render(humanTUISize(info.Size()))))
		return nil
	})
	if len(entries) > 500 {
		entries = append(entries[:500], tuiStyleMuted.Render("  … more entries omitted; use :tree for a bounded listing"))
	}
	return entries
}

func humanTUISize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func tuiMetric(label, value string) string { return fmt.Sprintf("  %-20s %s", label, value) }

func tuiBar(label string, value, total int, color lipgloss.Color) string {
	width := 28
	filled := value * width / maxTUI(total, 1)
	if value > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return fmt.Sprintf("  %-12s %s %d", label, lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled)+strings.Repeat("·", width-filled)), value)
}

func tuiString(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	text, _ := values[key].(string)
	return strings.TrimSpace(text)
}

func tuiStrings(values map[string]interface{}, key string) []string {
	items, _ := values[key].([]interface{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func tuiWrap(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len([]rune(line))+len([]rune(word))+1 > width {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func shortTUI(value string, limit int) string {
	if len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxTUI(1, limit-1)]) + "…"
}

func maxTUI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const (
	tuiColorCyan    lipgloss.Color = "86"
	tuiColorGreen   lipgloss.Color = "42"
	tuiColorMagenta lipgloss.Color = "170"
)

var (
	tuiStyleBrand      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	tuiStyleHeader     = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("238"))
	tuiStyleSidebarBox = lipgloss.NewStyle().Padding(1, 1).Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("238"))
	tuiStyleSidebar    = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	tuiStyleSelected   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	tuiStyleMain       = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	tuiStyleComposer   = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238"))
	tuiStyleFooter     = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Padding(0, 1)
	tuiStyleMuted      = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tuiStyleTitle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	tuiStyleSection    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	tuiStyleUser       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	tuiStyleAssistant  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	tuiStylePlan       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213"))
	tuiStyleTool       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	tuiStyleQuestion   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	tuiStyleError      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))
	tuiStyleStatus     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	tuiStyleStreaming  = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
)
