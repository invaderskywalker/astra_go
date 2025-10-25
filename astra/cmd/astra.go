// Command-line interface entrypoint for Astra CLI agent
package main

import (
	"astra/astra/agents/core"
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/sources/psql"
	"astra/astra/sources/psql/dao"
	colorutil "astra/astra/utils/color"
	"astra/astra/utils/logging"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if len(args) >= 1 && args[0] == "connect" {
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
		agent := core.NewBaseAgent(user.ID, sessionID, agentName, db.DB)

		logging.AppLogger.Info("Astra agent initialized in CLI",
			zap.String("dir", dirPath),
			zap.Int("userID", user.ID),
			zap.String("sessionID", sessionID),
		)

		// --- macOS Notification + Log Session ---
		sendMacNotification("🚀 Astra Agent Active", fmt.Sprintf("Session started in %s", dirPath))
		logSession(dirPath, sessionID, user.ID)

		// --- CLI Intro Message ---
		fmt.Printf("%s", colorutil.ColorPrompt("\n🧑‍🚀 Astra is now connected in this directory!\n\n"))
		fmt.Printf(colorutil.ColorInfo("Session: %s\nUser ID: %d\nPath: %s\n\n"), sessionID, user.ID, dirPath)
		fmt.Println(colorutil.ColorPrompt("You can:"))
		fmt.Println(colorutil.ColorInfo("  - Ask for project bootstrapping (e.g., 'Create a new Vite + TS + Three.js frontend here')"))
		fmt.Println(colorutil.ColorInfo("  - Request backend setup, schema generation, or debugging help"))
		fmt.Println(colorutil.ColorInfo("  - Chat about ideas or get coding help with real-time edits\n"))
		fmt.Println(colorutil.ColorPrompt("Type your command or 'exit' to quit.\n"))

		// --- Input Loop ---
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print(colorutil.ColorPrompt("astra> "))
			if !scanner.Scan() {
				break // EOF or error
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "exit" || line == "quit" {
				sendMacNotification("👋 Astra Disconnected", fmt.Sprintf("Session ended in %s", dirPath))
				fmt.Println(colorutil.ColorPrompt("👋 Goodbye!"))
				break
			}
			if line == "" {
				continue
			}

			outputCh := agent.ProcessQuery(line)
			for msg := range outputCh {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(msg), &data); err != nil {
					// If it's not JSON, just print as is (probably error or fallback)
					fmt.Print(colorutil.ColorWarning(msg))
					continue
				}
				eventType, _ := data["type"].(string)
				payload, _ := data["payload"].(map[string]interface{})

				switch eventType {
				case "error":
					if payload != nil {
						if msg, ok := payload["message"].(string); ok {
							fmt.Println(colorutil.ColorError(msg))
						} else {
							fmt.Println(colorutil.ColorError("Error occurred."))
						}
					}
				case "completed":
					// fmt.Println(colorutil.ColorFinalSuccess("\nProcess completed successfully!"))
					if payload != nil {
						if msg, ok := payload["message"].(string); ok {
							fmt.Println(colorutil.ColorFinalSuccess(msg))
						}
					}
				case "intermediate":
					// Show intermediate status/plan/step updates
					if payload != nil {
						if msg, ok := payload["message"].(string); ok {
							fmt.Println(colorutil.ColorInfo(msg))
						}
					}
				case "response_chunk":
					// if payload != nil {
					// 	if chunk, ok := payload["chunk"].(string); ok {
					// 		fmt.Print(colorutil.ColorAgentResponse(chunk))
					// 	}
					// }
				default:
					// fallback: print non-parsable, unexpected event as info
					// fmt.Println(colorutil.ColorInfo(msg))
				}
			}
			fmt.Println()
		}
		os.Exit(0)

	} else {
		fmt.Println(colorutil.ColorPrompt("Astra CLI usage:"))
		fmt.Println(colorutil.ColorInfo("  astra connect   # Connect to Astra agent in this directory"))
		os.Exit(1)
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
