// Command-line interface entrypoint for Astra CLI agent
package main

import (
	"astra/astra/agents/core"
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/sources/psql"
	"astra/astra/sources/psql/dao"
	"astra/astra/utils/logging"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
			// Not found → create new user
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

		// --- CLI Intro Message ---
		fmt.Printf("\n🧑‍🚀 Astra is now connected in this directory!\n\n")
		fmt.Printf("Session: %s\nUser ID: %d\nPath: %s\n\n", sessionID, user.ID, dirPath)
		fmt.Println("You can:")
		fmt.Println("  - Ask for project bootstrapping (e.g., 'Create a new Vite + TS + Three.js frontend here')")
		fmt.Println("  - Request backend setup, schema generation, or debugging help")
		fmt.Println("  - Chat about ideas or get coding help with real-time edits\n")
		fmt.Println("Type your command or 'exit' to quit.\n")

		// --- Input Loop ---
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
				if data["type"] != "response_chunk" {
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
