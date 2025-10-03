package routes

import (
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/utils/logging"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func AgentRoutes(ctrl *controllers.AgentsController, cfg config.Config) chi.Router {
	r := chi.NewRouter()

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			logging.ErrorLogger.Error("websocket accept error", zap.Error(err))
			http.Error(w, "failed to upgrade websocket", http.StatusInternalServerError)
			return
		}
		// defer conn.Close(websocket.StatusInternalError, "internal error")

		ctx := r.Context()

		// Read first message containing token and initial request
		typ, data, err := conn.Read(ctx)
		if err != nil {
			logging.ErrorLogger.Error("websocket read error", zap.Error(err))
			return
		}
		if typ != websocket.MessageText {
			conn.Close(websocket.StatusUnsupportedData, "unsupported data")
			return
		}

		var input struct {
			Token     string `json:"token"`
			AgentName string `json:"agent_name"`
			Query     string `json:"query"`
			SessionID string `json:"session_id"`
			UserID    int    `json:"user_id"`
		}
		if err := json.Unmarshal(data, &input); err != nil {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid json"}`))
			return
		}

		// Validate JWT
		token, err := jwt.Parse(input.Token, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid token"}`))
			conn.Close(websocket.StatusPolicyViolation, "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid claims"}`))
			conn.Close(websocket.StatusPolicyViolation, "invalid claims")
			return
		}

		userIDf, ok := claims["user_id"].(float64)
		if !ok {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid user_id"}`))
			conn.Close(websocket.StatusPolicyViolation, "invalid user_id")
			return
		}
		userID := int(userIDf)

		// Ensure user_id matches token
		if input.UserID != 0 && input.UserID != userID {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"user_id mismatch"}`))
			conn.Close(websocket.StatusPolicyViolation, "user_id mismatch")
			return
		}

		// Create initial AgentRequest
		agentReq := controllers.AgentRequest{
			AgentName: input.AgentName,
			Query:     input.Query,
			SessionID: input.SessionID,
			UserID:    userID, // Use validated userID
		}

		// Process initial request SYNCHRONOUSLY (sends response immediately)
		if !ctrl.ProcessAgentRequest(ctx, conn, &agentReq, userID) {
			logging.ErrorLogger.Error("failed to process initial request")
			return
		}

		// Now delegate further messages to AgentWebSocket (with validated userID)
		ctrl.AgentWebSocket(ctx, conn, userID)
	})

	return r
}
