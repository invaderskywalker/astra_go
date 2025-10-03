package routes

import (
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"astra/astra/utils/logging"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func AgentRoutes(ctrl *controllers.AgentsController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))
		gr.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
			if err != nil {
				logging.ErrorLogger.Error("websocket accept error", zap.Error(err))
				return
			}
			ctx := r.Context()
			ctrl.AgentWebSocket(ctx, conn)
		})
	})
	return r
}
