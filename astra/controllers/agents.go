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

func (c *AgentsController) AgentWebSocket(ctx context.Context, w *websocket.Conn) {
	defer w.Close(websocket.StatusInternalError, "internal error")

	for {
		typ, data, err := w.Read(ctx)
		if err != nil {
			logging.ErrorLogger.Error("websocket read error", zap.Error(err))
			return
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

		if req.SessionID == "" {
			req.SessionID = fmt.Sprintf("agent-%d-%s", req.UserID, time.Now().Format("20060102150405"))
		}

		agent := core.NewBaseAgent(req.UserID, req.SessionID, req.AgentName, c.db)
		respCh := agent.ProcessQuery(req.Query)

		for chunk := range respCh {
			if err := w.Write(ctx, websocket.MessageText, []byte(chunk)); err != nil {
				logging.ErrorLogger.Error("websocket write error", zap.Error(err))
				return
			}
		}
	}
}
