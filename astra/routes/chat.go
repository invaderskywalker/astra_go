package routes

import (
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"astra/astra/utils/logging"
	"astra/astra/utils/types"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"astra/astra/config"
)

func ChatRoutes(ctrl *controllers.ChatController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg)) // ✅ pass config
		gr.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req types.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			userID := r.Context().Value(middlewares.UserIDKey).(int)
			resp, err := ctrl.Chat(r.Context(), userID, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(resp)
		})
	})
	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			logging.Logger.Error("websocket accept error", "error", err)
			return
		}
		defer conn.Close(websocket.StatusInternalError, "internal error")

		ctx := r.Context()
		typ, data, err := conn.Read(ctx)
		if err != nil {
			logging.Logger.Error("websocket read error", "error", err)
			return
		}
		if typ != websocket.MessageText {
			conn.Close(websocket.StatusUnsupportedData, "unsupported data")
			return
		}

		var input struct {
			Token       string            `json:"token"`
			ChatRequest types.ChatRequest `json:"chat_request"`
		}
		if err := json.Unmarshal(data, &input); err != nil {
			conn.Write(ctx, websocket.MessageText, []byte(`{"error":"invalid json"}`))
			return
		}

		token, err := jwt.Parse(input.Token, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil // ✅ use cfg, not LoadConfig
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

		ch, errCh := ctrl.ChatStream(ctx, userID, input.ChatRequest)
		go func() {
			if err := <-errCh; err != nil {
				logging.Logger.Error("chat stream error", "error", err)
				conn.Write(ctx, websocket.MessageText, []byte(`{"error":"`+err.Error()+`"}`))
				conn.Close(websocket.StatusInternalError, "stream error")
			}
		}()

		for chunk := range ch {
			if err := conn.Write(ctx, websocket.MessageText, []byte(chunk)); err != nil {
				logging.Logger.Error("websocket write error", "error", err)
				return
			}
		}
		conn.Close(websocket.StatusNormalClosure, "")
	})
	return r
}
