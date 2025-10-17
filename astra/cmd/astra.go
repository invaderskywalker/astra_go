// Command-line interface entrypoint for Astra CLI agent
package main

import (
	"astra/astra/agents/core"
	"astra/astra/config"
	"astra/astra/sources/psql"
	"astra/astra/utils/logging"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
		logging.AppLogger.Info("Astra CLI: Connecting in directory", zap.String("dir", getWorkingDir()))

		db, err := psql.NewDatabase(ctx, cfg)
		if err != nil {
			logging.ErrorLogger.Error("database connection error", zap.Error(err))
			os.Exit(1)
		}
		defer db.Close()

		userID := 1
		sessionID := fmt.Sprintf("cli-%s", uuid.New().String()[:8]) // 🔥 unique session per run
		agentName := "astra-cli-agent"

		agent := core.NewBaseAgent(userID, sessionID, agentName, db.DB)
		logging.AppLogger.Info("Astra agent initialized in CLI",
			zap.String("dir", getWorkingDir()),
			zap.String("sessionID", sessionID),
		)

		fmt.Printf("\n🧑‍🚀 Astra is now connected in this directory!\n\n")
		fmt.Println("Session:", sessionID)
		fmt.Println()
		fmt.Println("You can:")
		fmt.Println("  - Ask for project bootstrapping (e.g., 'Create a new Vite + TS + Three.js frontend here')")
		fmt.Println("  - Request backend setup, schema generation, or debugging help")
		fmt.Println("  - Chat about ideas or get coding help with real-time edits\n")
		fmt.Println("Type your command or 'exit' to quit.\n")

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("astra> ")
			if !scanner.Scan() {
				break // EOF or error
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "exit" || line == "quit" {
				fmt.Println("👋 Goodbye!")
				break
			}
			if line == "" {
				continue
			}

			outputCh := agent.ProcessQuery(line)
			for msg := range outputCh {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(msg), &data); err != nil {
					fmt.Println(msg)
					continue
				}

				msgType, _ := data["type"].(string)
				if msgType != "response_chunk" {
					continue
				}

				payload, ok := data["payload"].(map[string]interface{})
				if !ok {
					continue
				}

				chunk, _ := payload["chunk"].(string)
				if chunk == "" {
					continue
				}

				// fmt.Print(chunk)
			}
			fmt.Println()
		}
		os.Exit(0)
	} else {
		fmt.Println("Astra CLI usage:")
		fmt.Println("  astra connect   # Connect to Astra agent in this directory")
		os.Exit(1)
	}
}

func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "<unknown>"
	}
	return wd
}
