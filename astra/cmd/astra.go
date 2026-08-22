// Command-line interface entrypoint for Astra CLI agent
package main

import (
	"astra/astra/agents/core"
	"astra/astra/agents/improvements"
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/evals"
	"astra/astra/services/llm"
	"astra/astra/sources/psql"
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/state"
	colorutil "astra/astra/utils/color"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/term"
)

func main() {
	// Initialize logger
	logging.InitLogger()
	cfg := config.LoadConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "--verison" {
		fmt.Println("Unknown option '--verison'. Did you mean '--version'?")
		return
	}
	if len(args) >= 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		fmt.Printf("Astra CLI v%s\n", config.CLIVersion)
		return
	}
	if len(args) >= 1 && args[0] == "improve" {
		runImprove(args[1:])
		return
	}
	if len(args) >= 1 && args[0] == "eval" {
		runEval(args[1:])
		return
	}
	if len(args) >= 1 && args[0] == "models" {
		printModels(ctx)
		return
	}
	if len(args) >= 1 && args[0] == "connect" {
		connectFlags := flag.NewFlagSet("connect", flag.ExitOnError)
		provider := connectFlags.String("provider", llm.DefaultProvider(), "LLM provider: ollama or openai")
		model := connectFlags.String("model", "", "model name, e.g. qwen3:14b or gpt-5.6-luna")
		plain := connectFlags.Bool("plain", false, "use the stream-friendly plain CLI instead of the full-screen cockpit")
		_ = connectFlags.Parse(args[1:])
		if *model == "" {
			*model = llm.DefaultModel(*provider)
		}
		dirPath := getWorkingDir()
		logging.AppLogger.Info("Astra CLI: Connecting in directory", zap.String("dir", dirPath))

		// --- Check for other running Astra processes ---
		activePaths := detectActiveAstraSessions()
		if len(activePaths) > 0 {
			msg := fmt.Sprintf("Astra already active in %d other place(s):\n", len(activePaths))
			for i, p := range activePaths {
				msg += fmt.Sprintf("  %d. %s\n", i+1, p)
			}
			sendMacNotification("Astra Active Elsewhere", msg)
			fmt.Printf(colorutil.ColorWarning("\nWarning: Astra already running in %d location(s):\n"), len(activePaths))
			for i, p := range activePaths {
				fmt.Printf(colorutil.ColorWarning("   %d. %s\n"), i+1, p)
			}
			fmt.Println()
		}

		// --- DB connection ---
		db, err := psql.NewDatabase(ctx, cfg)
		if err != nil {
			logging.ErrorLogger.Error("database connection error", zap.Error(err))
			os.Exit(1)
		}
		defer db.Close()

		// --- Setup DAO + Controller ---
		userDAO := dao.NewUserDAO(db.DB)
		userCtrl := controllers.NewUserController(userDAO)

		// --- Try to find or create user based on dir path ---
		user, err := userDAO.GetUserByUsername(ctx, dirPath)
		if err != nil {
			logging.ErrorLogger.Error("error fetching user", zap.Error(err))
			os.Exit(1)
		}
		if user == nil {
			email := fmt.Sprintf("%s@astra.local", filepath.Base(dirPath))
			user, err = userCtrl.CreateUser(ctx, dirPath, email, nil, nil)
			if err != nil {
				logging.ErrorLogger.Error("error creating user", zap.Error(err))
				os.Exit(1)
			}
			logging.AppLogger.Info("Created new Astra CLI user", zap.String("username", dirPath))
		} else {
			logging.AppLogger.Info("Found existing Astra CLI user", zap.Int("id", user.ID))
		}

		// --- Initialize agent ---
		sessionID := fmt.Sprintf("cli-%s", uuid.New().String())
		agentName := "astra"
		agent := core.NewBaseAgentWithWorkspace(user.ID, sessionID, agentName, db.DB, *provider, *model, dirPath)
		if _, manifestErr := state.EnsureSession(dirPath, user.ID, sessionID, *provider, *model); manifestErr != nil {
			logging.ErrorLogger.Warn("could not write session manifest", zap.Error(manifestErr))
		}

		logging.AppLogger.Info("Astra agent initialized in CLI",
			zap.String("dir", dirPath),
			zap.Int("userID", user.ID),
			zap.String("sessionID", sessionID), zap.String("provider", *provider), zap.String("model", *model),
		)

		// --- macOS Notification + Log Session ---
		sendMacNotification("🚀 Astra Agent Active", fmt.Sprintf("Session started in %s", dirPath))
		logSession(dirPath, sessionID, user.ID)

		// A real terminal gets the full-screen cockpit. Keeping the plain mode is
		// important for pipes, CI, transcript capture, and users who prefer raw
		// streaming output.
		if !*plain && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
			if err := runAstraTUI(agent, dirPath, cfg.MindPalaceRoot, *provider, *model); err != nil {
				logging.ErrorLogger.Warn("Astra cockpit exited with an error", zap.Error(err))
			}
			_ = state.CloseSession(dirPath, sessionID)
			sendMacNotification("👋 Astra Disconnected", fmt.Sprintf("Session ended in %s", dirPath))
			return
		}

		// --- CLI Intro Message ---
		fmt.Printf("%s", colorutil.ColorPrompt("\n🧑‍🚀 Astra is now connected in this directory!\n\n"))
		fmt.Printf(colorutil.ColorInfo("Session: %s\nUser ID: %d\nPath: %s\nModel: %s/%s\n\n"), sessionID, user.ID, dirPath, *provider, *model)
		fmt.Println(colorutil.ColorPrompt("You can:"))
		fmt.Println(colorutil.ColorInfo("  - Ask for project bootstrapping (e.g., 'Create a new Vite + TS + Three.js frontend here')"))
		fmt.Println(colorutil.ColorInfo("  - Request backend setup, schema generation, or debugging help"))
		fmt.Println(colorutil.ColorInfo("  - Chat about ideas or get coding help with real-time edits\n"))
		fmt.Println(colorutil.ColorPrompt("Type your command or 'exit' to quit."))
		fmt.Println(colorutil.ColorInfo("Enter send • paste multiline text then press Enter once • Ctrl-J new line • Ctrl-W delete word • Ctrl-U clear draft • Ctrl-C cancel draft\n"))

		// --- Interactive input + output multiplexer ---
		// Input is read independently, so a new request can be submitted while a
		// previous one is planning, editing, testing, or streaming its response.
		inputCh := interactiveInput(colorutil.ColorPrompt("astra> "))
		type outputEvent struct {
			id      int
			message string
			done    bool
		}
		eventCh := make(chan outputEvent, 128)
		nextID := 1
		active := 0
		pasteMode := false
		pasteLines := []string{}
		queueQuery := func(line string) {
			if len(line) > 12000 {
				line = saveLargePaste(dirPath, line)
			}
			id, outputCh := nextID, agent.ProcessQuery(line)
			nextID++
			active++
			go func(id int, outputCh <-chan string) {
				for message := range outputCh {
					eventCh <- outputEvent{id: id, message: message}
				}
				eventCh <- outputEvent{id: id, done: true}
			}(id, outputCh)
			fmt.Print(fmt.Sprintf(colorutil.ColorInfo("Queued request #%d (you can continue typing).\r\n"), id))
		}
		for inputCh != nil || active > 0 {
			cases := []reflect.SelectCase{{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(inputCh)}}
			if inputCh == nil {
				cases = nil
			}
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(eventCh)})
			chosen, value, ok := reflect.Select(cases)
			if inputCh != nil && chosen == 0 {
				if !ok {
					inputCh = nil
					continue
				}
				line := strings.TrimSpace(value.String())
				if pasteMode {
					if line == ":endpaste" {
						pasteMode = false
						queueQuery(strings.Join(pasteLines, "\n"))
						pasteLines = nil
					} else {
						pasteLines = append(pasteLines, value.String())
					}
					continue
				}
				if line == "" {
					continue
				}
				if line == "exit" || line == "quit" {
					_ = state.CloseSession(dirPath, sessionID)
					sendMacNotification("👋 Astra Disconnected", fmt.Sprintf("Session ended in %s", dirPath))
					fmt.Println(colorutil.ColorPrompt("👋 Goodbye!"))
					break
				}
				if line == ":paste" {
					pasteMode = true
					pasteLines = nil
					fmt.Println(colorutil.ColorInfo("Paste content now. Type :endpaste on its own line when finished."))
					continue
				}
				handled, nextProvider, nextModel := handleCLICommand(line, dirPath, *provider, *model, agent, active, cfg.MindPalaceRoot)
				if handled {
					*provider, *model = nextProvider, nextModel
					continue
				}
				queueQuery(line)
				continue
			}
			if chosen == len(cases)-1 {
				event, _ := value.Interface().(outputEvent)
				if event.done {
					active--
					if active == 0 {
						fmt.Println()
					}
					continue
				}
				printCLIEvent(event.message)
			}
		}
		_ = state.CloseSession(dirPath, sessionID)
		restoreInteractiveTerminal()
		os.Exit(0)

	} else {
		fmt.Println(colorutil.ColorPrompt("Astra CLI usage:"))
		fmt.Println(colorutil.ColorInfo("  astra --version"))
		fmt.Println(colorutil.ColorInfo("  astra version"))
		fmt.Println(colorutil.ColorInfo("  astra connect [--provider ollama|openai] [--model MODEL]"))
		fmt.Println(colorutil.ColorInfo("                 [--plain]  # use the automation-friendly stream CLI"))
		fmt.Println(colorutil.ColorInfo("  astra models    # Show local Ollama and supported cloud model choices"))
		fmt.Println(colorutil.ColorInfo("  astra eval list|local [--root DIR]  # Run deterministic capability evaluations"))
		fmt.Println(colorutil.ColorInfo("  astra improve scan|list|review|approve|reject  # Improvement proposal queue"))
		os.Exit(1)
	}
}

func runEval(args []string) {
	if len(args) == 0 || args[0] == "list" {
		fmt.Println(colorutil.ColorPrompt("Astra evaluation scenarios:"))
		for _, scenario := range evals.BuiltinScenarios {
			fmt.Printf("  %-24s [%s] %s\n", scenario.ID, scenario.Category, scenario.Name)
		}
		fmt.Println(colorutil.ColorInfo("Run `astra eval local` for deterministic action, artifact, memory, and prompt checks."))
		return
	}
	if args[0] != "local" {
		fmt.Println("Usage: astra eval list|local [--root DIR]")
		return
	}
	flags := flag.NewFlagSet("eval local", flag.ContinueOnError)
	root := flags.String("root", "", "temporary evaluation workspace; defaults to a new temp directory")
	if err := flags.Parse(args[1:]); err != nil {
		fmt.Println(colorutil.ColorError("Evaluation options: " + err.Error()))
		return
	}
	evaluationRoot := strings.TrimSpace(*root)
	if evaluationRoot == "" {
		var err error
		evaluationRoot, err = os.MkdirTemp("", "astra-eval-")
		if err != nil {
			fmt.Println(colorutil.ColorError("Could not create evaluation workspace: " + err.Error()))
			return
		}
	}
	report := evals.RunLocal(evaluationRoot)
	fmt.Printf("Astra local evaluation: %d passed, %d failed\n", report.Passed, report.Failed)
	fmt.Println("Evaluation root: " + evaluationRoot)
	for _, check := range report.Checks {
		prefix := "✓"
		if !check.Passed {
			prefix = "✗"
		}
		line := fmt.Sprintf("%s %s — %s", prefix, check.ID, check.Summary)
		if check.Evidence != "" {
			line += " (" + check.Evidence + ")"
		}
		if check.Passed {
			fmt.Println(colorutil.ColorFinalSuccess(line))
		} else {
			fmt.Println(colorutil.ColorError(line))
		}
	}
}

func runImprove(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: astra improve scan|list|review|approve|reject")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	switch args[0] {
	case "scan":
		flags := flag.NewFlagSet("improve scan", flag.ExitOnError)
		root := flags.String("root", ".astra/improvements", "proposal queue directory")
		provider := flags.String("provider", "ollama", "LLM provider")
		model := flags.String("model", "qwen3:14b", "scout model")
		_ = flags.Parse(args[1:])
		proposal, err := improvements.Scan(ctx, llm.NewClient(*provider), *model, getWorkingDir())
		if err != nil {
			fmt.Println(colorutil.ColorError("Scan failed: " + err.Error()))
			return
		}
		proposal, err = improvements.New(*root).SaveProposal(proposal)
		if err != nil {
			fmt.Println(colorutil.ColorError("Could not save proposal: " + err.Error()))
			return
		}
		fmt.Println(colorutil.ColorFinalSuccess("Proposal created: " + proposal.ID))
		fmt.Println(proposal.Title)
	case "list":
		flags := flag.NewFlagSet("improve list", flag.ExitOnError)
		root := flags.String("root", ".astra/improvements", "proposal queue directory")
		_ = flags.Parse(args[1:])
		proposals, err := improvements.New(*root).List()
		if err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
			return
		}
		if len(proposals) == 0 {
			fmt.Println("No improvement proposals.")
			return
		}
		for _, proposal := range proposals {
			fmt.Printf("%s  [%s]  %s\n", proposal.ID, proposal.Status, proposal.Title)
		}
	case "review":
		flags := flag.NewFlagSet("improve review", flag.ExitOnError)
		root := flags.String("root", ".astra/improvements", "proposal queue directory")
		provider := flags.String("provider", "openai", "LLM provider")
		model := flags.String("model", "gpt-5.6-luna", "review model")
		_ = flags.Parse(args[1:])
		if flags.NArg() != 1 {
			fmt.Println("Usage: astra improve review <proposal-id>")
			return
		}
		store := improvements.New(*root)
		proposal, err := store.Get(flags.Arg(0))
		if err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
			return
		}
		review, err := improvements.ReviewProposal(ctx, llm.NewClient(*provider), *model, proposal)
		if err != nil {
			fmt.Println(colorutil.ColorError("Review failed: " + err.Error()))
			return
		}
		if err := store.SaveReview(review); err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
			return
		}
		fmt.Printf("%s: %s\n%s\n", review.Recommendation, proposal.Title, review.Rationale)
	case "approve", "reject":
		flags := flag.NewFlagSet("improve "+args[0], flag.ExitOnError)
		root := flags.String("root", ".astra/improvements", "proposal queue directory")
		reason := flags.String("reason", "", "human decision reason")
		_ = flags.Parse(args[1:])
		if flags.NArg() != 1 {
			fmt.Printf("Usage: astra improve %s <proposal-id> [--reason TEXT]\n", args[0])
			return
		}
		store := improvements.New(*root)
		status := improvements.Approved
		recommendation := "approved"
		if args[0] == "reject" {
			status, recommendation = improvements.Rejected, "rejected"
		}
		proposal, err := store.SetStatus(flags.Arg(0), status)
		if err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
			return
		}
		_ = store.SaveReview(improvements.Review{ProposalID: proposal.ID, Model: "human", Recommendation: recommendation, Rationale: *reason})
		fmt.Printf("Proposal %s %s. No code has been changed.\n", proposal.ID, recommendation)
	default:
		fmt.Println("Usage: astra improve scan|list|review|approve|reject")
	}
}

func printModels(ctx context.Context) {
	fmt.Println(colorutil.ColorPrompt("Model choices"))
	fmt.Println(colorutil.ColorInfo("OpenAI: gpt-5.6-luna (efficient), gpt-5.6-terra (balanced), gpt-5.6-sol (flagship)"))
	models, err := llm.ListOllamaModels(ctx)
	if err != nil {
		fmt.Println(colorutil.ColorWarning("Ollama: unavailable (" + err.Error() + ")"))
		return
	}
	if len(models) == 0 {
		fmt.Println(colorutil.ColorWarning("Ollama: no local models installed"))
		return
	}
	fmt.Println(colorutil.ColorInfo("Ollama:"))
	for _, model := range models {
		fmt.Println(colorutil.ColorInfo("  - " + model))
	}
}

func handleCLICommand(line, dir, provider, model string, agent *core.BaseAgent, active int, roots ...string) (bool, string, string) {
	if !strings.HasPrefix(line, ":") {
		return false, provider, model
	}
	parts := strings.Fields(line)
	command := strings.TrimPrefix(parts[0], ":")
	memoryRoot := ""
	if len(roots) > 0 {
		memoryRoot = roots[0]
	}
	switch command {
	case "help":
		fmt.Println(colorutil.ColorInfo("Views: :chat, :dashboard, :workspace, :mindpalace, :sessions, :sync"))
		fmt.Println(colorutil.ColorInfo("Local commands: :pwd, :ls [path], :tree [path], :attach <file>, :paste/:endpaste, :model, :pause, :resume, :stop, :clear, :help, :quit"))
	case "chat":
		printCLIText(colorutil.ColorPrompt("Chat mode active. Type a request or use :dashboard, :workspace, :mindpalace, :sessions, or :sync."))
	case "dashboard":
		printDashboard(dir, memoryRoot, provider, model, agent, active)
	case "workspace":
		printWorkspaceView(dir)
	case "mindpalace", "memory":
		printMindPalaceView(memoryRoot, agent.UserID)
	case "sessions", "session":
		printSessionsView(dir, memoryRoot, agent.UserID)
	case "sync":
		printSyncView(dir, memoryRoot)
	case "pwd":
		fmt.Println(dir)
	case "model":
		if len(parts) == 3 {
			if active > 0 {
				fmt.Println(colorutil.ColorWarning("Wait for current requests to finish before switching models."))
			} else if err := agent.SetModel(parts[1], parts[2]); err != nil {
				fmt.Println(colorutil.ColorError(err.Error()))
			} else {
				provider, model = parts[1], parts[2]
				fmt.Printf(colorutil.ColorFinalSuccess("Switched to %s/%s for future requests.\n"), provider, model)
			}
		} else {
			fmt.Printf(colorutil.ColorInfo("Current model: %s/%s\nUsage: :model <ollama|openai> <model>\n"), provider, model)
		}
	case "pause":
		agent.Pause()
		fmt.Println(colorutil.ColorWarning("Pause requested. Astra will pause at the next safe checkpoint."))
	case "resume":
		agent.Resume()
		fmt.Println(colorutil.ColorFinalSuccess("Astra resumed."))
	case "stop", "cancel":
		if agent.Stop() {
			fmt.Println(colorutil.ColorWarning("Stop requested. Astra will cancel the active request safely."))
		} else {
			fmt.Println(colorutil.ColorInfo("No active request to stop."))
		}
	case "clear", "abort":
		stopped := agent.Stop()
		cleared := agent.ClearPending()
		if stopped || cleared > 0 {
			fmt.Printf(colorutil.ColorWarning("Cleared %d queued request(s); active request cancellation requested.\n"), cleared)
		} else {
			fmt.Println(colorutil.ColorInfo("No active or queued requests to clear."))
		}
	case "ls", "tree":
		path := dir
		if len(parts) > 1 {
			var err error
			path, err = workspaceCLIPath(dir, parts[1])
			if err != nil {
				fmt.Println(colorutil.ColorError(err.Error()))
				break
			}
		}
		if err := listCLIFiles(path, dir, command == "tree", 0); err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
		}
	case "attach":
		if len(parts) != 2 {
			fmt.Println(colorutil.ColorWarning("Usage: :attach <file-path>"))
			break
		}
		if target, err := attachCLIFile(dir, parts[1]); err != nil {
			fmt.Println(colorutil.ColorError(err.Error()))
		} else {
			fmt.Println(colorutil.ColorFinalSuccess("Attached: " + target))
		}
	default:
		fmt.Println(colorutil.ColorWarning("Unknown local command. Type :help."))
	}
	return true, provider, model
}

func workspaceCLIPath(root, requested string) (string, error) {
	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must remain inside the connected workspace; use :attach for an outside file")
	}
	return path, nil
}

func listCLIFiles(path, root string, recursive bool, depth int) error {
	if depth > 6 {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".git") || entry.Name() == "node_modules" {
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Join(path, entry.Name()))
		prefix := strings.Repeat("  ", depth)
		if entry.IsDir() {
			fmt.Printf("%s📁 %s/\n", prefix, rel)
			if recursive {
				if err := listCLIFiles(filepath.Join(path, entry.Name()), root, true, depth+1); err != nil {
					return err
				}
			}
		} else {
			info, _ := entry.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			fmt.Printf("%s📄 %s (%d bytes)\n", prefix, rel, size)
		}
	}
	return nil
}

func attachCLIFile(root, requested string) (string, error) {
	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("attachments must be files")
	}
	if info.Size() > 32*1024*1024 {
		return "", fmt.Errorf("file is larger than 32 MB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	name := filepath.Base(path)
	destination := filepath.Join(root, ".astra", "attachments", uuid.New().String()+"-"+name)
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(destination, data, 0644); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(".astra", "attachments", filepath.Base(destination))), nil
}

func saveLargePaste(root, content string) string {
	destination := filepath.Join(root, ".astra", "attachments", "paste-"+uuid.New().String()+".txt")
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "I received a large paste, but could not save it: " + err.Error()
	}
	if err := os.WriteFile(destination, []byte(content), 0644); err != nil {
		return "I received a large paste, but could not save it: " + err.Error()
	}
	rel, _ := filepath.Rel(root, destination)
	return fmt.Sprintf("I pasted a large input. Read the saved attachment at %s and use it as the source for this request.", filepath.ToSlash(rel))
}

func printCLIEvent(message string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(message), &data); err != nil {
		printCLIText(colorutil.ColorWarning(message))
		return
	}
	eventType, _ := data["type"].(string)
	payload, _ := data["payload"].(map[string]interface{})
	switch eventType {
	case "plan":
		printPlan(payload)
	case "status":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorInfo("• " + text))
			}
		}
	case "needs_input":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorWarning(text))
			}
			if questions, ok := payload["questions"].([]interface{}); ok {
				for _, question := range questions {
					if text, ok := question.(string); ok {
						printCLIText(colorutil.ColorWarning("? " + text))
					}
				}
			}
		}
	case "action_plan":
		printActionPlan(payload)
	case "action_activation":
		if payload != nil {
			actions, _ := payload["actions"].([]interface{})
			names := make([]string, 0, len(actions))
			for _, raw := range actions {
				if name, ok := raw.(string); ok && strings.TrimSpace(name) != "" {
					names = append(names, name)
				}
			}
			if len(names) > 0 {
				printCLIText(colorutil.ColorInfo("↳ Loaded tool documentation: " + strings.Join(names, ", ")))
			}
		}
	case "action_result":
		printActionResult(payload)
	case "error":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorError(text))
			}
		}
	case "paused":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorWarning("⏸ " + text))
			}
		}
	case "stopped":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorWarning("■ " + text))
			}
		}
	case "completed":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorFinalSuccess("✓ " + text))
			}
		}
	case "intermediate":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				printCLIText(colorutil.ColorInfo(text))
			}
		}
	case "response_chunk":
		if payload != nil {
			if text, ok := payload["chunk"].(string); ok {
				// Raw terminal mode does not translate LF into a carriage return.
				// Normalize streamed model text at the output boundary so wrapped
				// lines always start at column zero.
				printCLIStream(colorutil.ColorAgentResponse(text))
			}
		}
	}
}

func normalizeTerminalText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.ReplaceAll(text, "\n", "\r\n")
}

func printCLIText(text string) {
	fmt.Print("\r\n" + normalizeTerminalText(text) + "\r\n")
}

func printCLIStream(text string) {
	fmt.Print(normalizeTerminalText(text))
}

func printPlan(plan map[string]interface{}) {
	if plan == nil {
		return
	}
	fmt.Print(colorutil.ColorPrompt("\r\n┌─ Astra plan\r\n"))
	if mode := valueString(plan["mode"]); mode != "" {
		fmt.Print(colorutil.ColorInfo("│ Mode: " + mode + "\r\n"))
	}
	if goal := valueString(plan["goal"]); goal != "" {
		printWrappedCLI("│ Goal: ", goal, "│       ")
	}
	if skills := valueStrings(plan["selected_skills"]); len(skills) > 0 {
		printWrappedCLI("│ Skills: ", strings.Join(skills, ", "), "│         ")
	}
	if criteria := valueStrings(plan["success_criteria"]); len(criteria) > 0 {
		fmt.Print(colorutil.ColorInfo("│ Success criteria:\r\n"))
		for _, item := range criteria {
			printWrappedCLI("│   ✓ ", item, "│     ")
		}
	}
	if steps := valueStrings(plan["mind_map_steps_in_natural_language"]); len(steps) > 0 {
		fmt.Print(colorutil.ColorInfo("│ Mind map:\r\n"))
		for i, step := range steps {
			printWrappedCLI(fmt.Sprintf("│   %d. ", i+1), step, "│      ")
		}
	}
	if verification := valueStrings(plan["verification"]); len(verification) > 0 {
		fmt.Print(colorutil.ColorInfo("│ Verification:\r\n"))
		for _, item := range verification {
			printWrappedCLI("│   • ", item, "│     ")
		}
	}
	if risks := valueStrings(plan["risks"]); len(risks) > 0 {
		fmt.Print(colorutil.ColorWarning("│ Risks / assumptions to watch:\r\n"))
		for _, item := range risks {
			printWrappedCLI("│   ! ", item, "│     ")
		}
	}
	if stops := valueStrings(plan["stop_conditions"]); len(stops) > 0 {
		fmt.Print(colorutil.ColorInfo("│ Stop conditions:\r\n"))
		for _, item := range stops {
			printWrappedCLI("│   • ", item, "│     ")
		}
	}
	fmt.Print(colorutil.ColorPrompt("└─\r\n"))
}

func printActionPlan(payload map[string]interface{}) {
	if payload == nil {
		return
	}
	step, _ := payload["step"].(map[string]interface{})
	if step == nil {
		return
	}
	index := int(valueNumber(payload["index"]))
	action := valueString(step["action"])
	label := fmt.Sprintf("  ├─ Action %d", index)
	if action != "" {
		label += ": " + action
	}
	fmt.Print(colorutil.ColorInfo(label + "\r\n"))
	if reason := valueString(step["reason"]); reason != "" {
		printWrappedCLI("  │  Why: ", reason, "  │       ")
	}
	if expected := valueString(step["expected_observation"]); expected != "" {
		printWrappedCLI("  │  Expect: ", expected, "  │          ")
	}
	if root := valueString(payload["workspace_root"]); root != "" {
		printWrappedCLI("  │  Scope: ", root, "  │         ")
	}
	if params := step["action_params"]; params != nil {
		encoded, err := json.Marshal(params)
		if err == nil && string(encoded) != "{}" {
			text := string(encoded)
			if len(text) > 1400 {
				text = text[:1400] + "…"
			}
			printWrappedCLI("  │  Params: ", text, "  │          ")
		}
	}
}

func printActionResult(payload map[string]interface{}) {
	if payload == nil {
		return
	}
	action := valueString(payload["action"])
	results, _ := payload["result"].(map[string]interface{})
	for _, raw := range results {
		entry, _ := raw.(map[string]interface{})
		if entry == nil {
			continue
		}
		success, _ := entry["success"].(bool)
		prefix := "✓ "
		if !success {
			prefix = "✗ "
		}
		message := valueString(entry["summary"])
		if message == "" {
			message = valueString(entry["error"])
		}
		if message == "" {
			message = "completed"
		}
		if workingDirectory := valueString(entry["working_directory"]); workingDirectory != "" {
			message += " @ " + workingDirectory
		}
		line := prefix + action + ": " + message
		if success {
			printCLIText(colorutil.ColorFinalSuccess(line))
		} else {
			printCLIText(colorutil.ColorError(line))
		}
		if commands, ok := entry["commands"].([]interface{}); ok {
			for _, rawCommand := range commands {
				command, _ := rawCommand.(map[string]interface{})
				if command == nil {
					continue
				}
				commandLine := fmt.Sprintf("  %s (%s)", valueString(command["command"]), valueString(command["working_directory"]))
				if errText := valueString(command["error"]); errText != "" {
					commandLine += " → " + errText
				} else if stdout := valueString(command["stdout"]); stdout != "" {
					commandLine += " → " + strings.ReplaceAll(stdout, "\r\n", " ")
				}
				printCLIText(colorutil.ColorInfo(commandLine))
			}
		}
	}
}

func printWrappedCLI(prefix, text, continuation string) {
	words := strings.Fields(text)
	if len(words) == 0 {
		fmt.Print(colorutil.ColorInfo(prefix + "\r\n"))
		return
	}
	line := prefix
	const width = 108
	for _, word := range words {
		if len([]rune(line))+len([]rune(word))+1 > width && line != prefix {
			fmt.Print(colorutil.ColorInfo(line + "\r\n"))
			line = continuation + word
			continue
		}
		if line != prefix {
			line += " "
		}
		line += word
	}
	fmt.Print(colorutil.ColorInfo(line + "\r\n"))
}

func valueString(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func valueStrings(value interface{}) []string {
	items, _ := value.([]interface{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := valueString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func valueNumber(value interface{}) float64 {
	number, _ := value.(float64)
	return number
}

// --- Helper: Get Working Directory ---
func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "<unknown>"
	}
	return wd
}

// --- Helper: macOS Notification ---
func sendMacNotification(title, message string) {
	cmd := exec.Command("osascript", "-e", fmt.Sprintf(`display notification \"%s\" with title \"%s\"`, escapeAppleScript(message), escapeAppleScript(title)))
	_ = cmd.Run()
}

// --- Helper: Escape for AppleScript ---
func escapeAppleScript(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// --- Helper: Log Session Info ---
func logSession(dirPath, sessionID string, userID int) {
	homeDir, _ := os.UserHomeDir()
	logFile := filepath.Join(homeDir, ".astra_sessions.log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] UserID=%d | Session=%s | Path=%s\n", timestamp, userID, sessionID, dirPath)
	f.WriteString(entry)
}

// --- Helper: Detect Other Running Astra Instances ---
func detectActiveAstraSessions() []string {
	// 1. Use pgrep to find all running astra processes
	out, err := exec.Command("pgrep", "-fl", "astra").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var paths []string
	for _, line := range lines {
		if strings.Contains(line, "astra connect") {
			// try to extract working directory (from command path if possible)
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "/") && strings.Contains(p, "astra") {
					paths = append(paths, filepath.Dir(p))
					break
				}
			}
		}
	}
	return paths
}
