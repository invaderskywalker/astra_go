// Command-line interface entrypoint for Astra CLI agent
package main

import (
	"astra/astra/agents/core"
	"astra/astra/agents/improvements"
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/services/llm"
	"astra/astra/sources/psql"
	"astra/astra/sources/psql/dao"
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
	if len(args) >= 1 && args[0] == "models" {
		printModels(ctx)
		return
	}
	if len(args) >= 1 && args[0] == "connect" {
		connectFlags := flag.NewFlagSet("connect", flag.ExitOnError)
		provider := connectFlags.String("provider", llm.DefaultProvider(), "LLM provider: ollama or openai")
		model := connectFlags.String("model", "", "model name, e.g. qwen3:14b or gpt-5.6-luna")
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
		agent := core.NewBaseAgentWithModel(user.ID, sessionID, agentName, db.DB, *provider, *model)

		logging.AppLogger.Info("Astra agent initialized in CLI",
			zap.String("dir", dirPath),
			zap.Int("userID", user.ID),
			zap.String("sessionID", sessionID), zap.String("provider", *provider), zap.String("model", *model),
		)

		// --- macOS Notification + Log Session ---
		sendMacNotification("🚀 Astra Agent Active", fmt.Sprintf("Session started in %s", dirPath))
		logSession(dirPath, sessionID, user.ID)

		// --- CLI Intro Message ---
		fmt.Printf("%s", colorutil.ColorPrompt("\n🧑‍🚀 Astra is now connected in this directory!\n\n"))
		fmt.Printf(colorutil.ColorInfo("Session: %s\nUser ID: %d\nPath: %s\nModel: %s/%s\n\n"), sessionID, user.ID, dirPath, *provider, *model)
		fmt.Println(colorutil.ColorPrompt("You can:"))
		fmt.Println(colorutil.ColorInfo("  - Ask for project bootstrapping (e.g., 'Create a new Vite + TS + Three.js frontend here')"))
		fmt.Println(colorutil.ColorInfo("  - Request backend setup, schema generation, or debugging help"))
		fmt.Println(colorutil.ColorInfo("  - Chat about ideas or get coding help with real-time edits\n"))
		fmt.Println(colorutil.ColorPrompt("Type your command or 'exit' to quit."))
		fmt.Println(colorutil.ColorInfo("Enter send • Ctrl-J new line • Ctrl-W delete word • Ctrl-U clear draft • Ctrl-C cancel draft\n"))

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
				handled, nextProvider, nextModel := handleCLICommand(line, dirPath, *provider, *model, agent, active)
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
		restoreInteractiveTerminal()
		os.Exit(0)

	} else {
		fmt.Println(colorutil.ColorPrompt("Astra CLI usage:"))
		fmt.Println(colorutil.ColorInfo("  astra --version"))
		fmt.Println(colorutil.ColorInfo("  astra version"))
		fmt.Println(colorutil.ColorInfo("  astra connect [--provider ollama|openai] [--model MODEL]"))
		fmt.Println(colorutil.ColorInfo("  astra models    # Show local Ollama and supported cloud model choices"))
		fmt.Println(colorutil.ColorInfo("  astra improve scan|list|review|approve|reject  # Improvement proposal queue"))
		os.Exit(1)
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

func handleCLICommand(line, dir, provider, model string, agent *core.BaseAgent, active int) (bool, string, string) {
	if !strings.HasPrefix(line, ":") {
		return false, provider, model
	}
	parts := strings.Fields(line)
	command := strings.TrimPrefix(parts[0], ":")
	switch command {
	case "help":
		fmt.Println(colorutil.ColorInfo("Local commands: :pwd, :ls [path], :tree [path], :attach <file>, :paste/:endpaste, :model, :help, :quit"))
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
	message = strings.ReplaceAll(message, "\n", "\r\n")
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(message), &data); err != nil {
		fmt.Print(colorutil.ColorWarning(message))
		return
	}
	eventType, _ := data["type"].(string)
	payload, _ := data["payload"].(map[string]interface{})
	switch eventType {
	case "error":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				fmt.Print(colorutil.ColorError(text) + "\r\n")
			}
		}
	case "completed":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				fmt.Print(colorutil.ColorFinalSuccess(text) + "\r\n")
			}
		}
	case "intermediate":
		if payload != nil {
			if text, ok := payload["message"].(string); ok {
				fmt.Print(colorutil.ColorInfo(text) + "\r\n")
			}
		}
	case "response_chunk":
		if payload != nil {
			if text, ok := payload["chunk"].(string); ok {
				fmt.Print(colorutil.ColorAgentResponse(text))
			}
		}
	}
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
