package routes

import (
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"astra/astra/types"
	"encoding/json"
	"net/http"
	"strconv"

	"astra/astra/config"

	"github.com/go-chi/chi/v5"
)

func UserRoutes(ctrl *controllers.UserController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg)) // ✅ pass config
		gr.Get("/fetch/{user_id}", func(w http.ResponseWriter, r *http.Request) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				http.Error(w, "invalid user_id", http.StatusBadRequest)
				return
			}
			user, err := ctrl.GetUser(r.Context(), id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(user)
		})
		gr.Get("/fetch", func(w http.ResponseWriter, r *http.Request) {
			users, err := ctrl.GetAllUsers(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(users)
		})
	})
	r.Post("/create", func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		user, err := ctrl.CreateUser(r.Context(), req.Username, req.Email, req.FullName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(user)
	})
	return r
}
