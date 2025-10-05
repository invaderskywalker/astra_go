package controllers

import (
	"astra/astra/agents/core"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type AgentsController struct {
	db *gorm.DB
}

func NewAgentsController(db *gorm.DB) *AgentsController {
	return &AgentsController{db: db}
}

type AgentRequest struct {
	AgentName string `json:"agent_name"`
	Query     string `json:"query"`
	SessionID string `json:"session_id"`
	UserID    int    `json:"user_id"`
}

// ProcessAgentRequest handles a single agent query and writes responses to the connection.
// Returns true if processing was successful.
func (c *AgentsController) ProcessAgentRequest(ctx context.Context, w *websocket.Conn, req *AgentRequest, validatedUserID int) bool {
	if req.UserID != validatedUserID {
		w.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid user_id"}`))
		return false
	}

	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("agent-%d-%s", validatedUserID, time.Now().Format("20060102150405"))
	}

	// Special handling for "init" query
	if req.Query == "init" {
		ack := `{"status":"connected","session_id":"` + req.SessionID + `"}`
		if err := w.Write(ctx, websocket.MessageText, []byte(ack)); err != nil {
			logging.ErrorLogger.Error("websocket write ack error", zap.Error(err))
			return false
		}
		return true
	}

	agent := core.NewBaseAgent(validatedUserID, req.SessionID, req.AgentName, c.db)
	respCh := agent.ProcessQuery(req.Query)

	for chunk := range respCh {
		if err := w.Write(ctx, websocket.MessageText, []byte(chunk)); err != nil {
			logging.ErrorLogger.Error("websocket write error", zap.Error(err))
			return false
		}
	}
	return true
}

func (c *AgentsController) AgentWebSocket(ctx context.Context, w *websocket.Conn, validatedUserID int) {
	// Set up ping/pong to keep connection alive
	pingInterval := 30 * time.Second
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	// Create a context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	defer func() {
		if ctx.Err() != nil {
			w.Close(websocket.StatusNormalClosure, "context cancelled")
		}
	}()

	// Goroutine to send periodic pings
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := w.Ping(ctx); err != nil {
					logging.ErrorLogger.Error("websocket ping error", zap.Error(err))
					cancel() // Cancel context on ping failure
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			logging.AppLogger.Info("websocket context done, closing connection")
			return
		default:
			typ, data, err := w.Read(ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					logging.AppLogger.Info("client closed connection")
					return
				}
				logging.ErrorLogger.Error("websocket read error", zap.Error(err))
				time.Sleep(1 * time.Second) // give client time to reconnect
				continue
			}

			if typ != websocket.MessageText {
				w.Write(ctx, websocket.MessageText, []byte(`{"error":"unsupported data"}`))
				continue
			}

			var req AgentRequest
			if err := json.Unmarshal(data, &req); err != nil {
				w.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid json"}`))
				continue
			}

			// Process the request (validates user_id and handles session/init)
			if !c.ProcessAgentRequest(ctx, w, &req, validatedUserID) {
				continue // Or close on repeated failures if needed
			}
		}
	}
}
