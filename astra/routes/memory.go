package routes

import (
	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func MemoryRoutes(ctrl *controllers.MemoryController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))
		gr.Get("/fetch/{user_id}", func(w http.ResponseWriter, r *http.Request) { serveMemory(w, r, ctrl, "") })
		gr.Get("/fetch/{user_id}/type/{kind}", func(w http.ResponseWriter, r *http.Request) { serveMemory(w, r, ctrl, chi.URLParam(r, "kind")) })
	})
	return r
}

func serveMemory(w http.ResponseWriter, r *http.Request, ctrl *controllers.MemoryController, kind string) {
	userID, err := strconv.Atoi(chi.URLParam(r, "user_id"))
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}
	records, err := ctrl.List(r.Context(), userID, kind)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(records)
}
