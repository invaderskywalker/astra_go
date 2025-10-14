package routes

import (
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"astra/astra/utils/types"
	"encoding/json"
	"net/http"
	"strconv"

	"astra/astra/config"

	"github.com/go-chi/chi/v5"
)

// generic wrapper to reduce boilerplate
func handleJSON(handler func(r *http.Request) (any, int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, status, err := handler(r)
		if err != nil {
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(res)
	}
}

func UserRoutes(ctrl *controllers.UserController, cfg config.Config) chi.Router {
	r := chi.NewRouter()

	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))

		gr.Get("/fetch/{user_id}", handleJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			user, err := ctrl.GetUser(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return user, http.StatusOK, nil
		}))

		gr.Get("/me", handleJSON(func(r *http.Request) (any, int, error) {
			userIDVal := r.Context().Value(middlewares.UserIDKey)
			id, ok := userIDVal.(int)
			if !ok {
				return nil, http.StatusUnauthorized, nil
			}
			user, err := ctrl.GetUser(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return user, http.StatusOK, nil
		}))

		gr.Put("/me", handleJSON(func(r *http.Request) (any, int, error) {
			userIDVal := r.Context().Value(middlewares.UserIDKey)
			id, ok := userIDVal.(int)
			if !ok {
				return nil, http.StatusUnauthorized, nil
			}
			var req types.UpdateUserRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return nil, http.StatusBadRequest, err
			}
			user, err := ctrl.UpdateUser(r.Context(), id, req.Username, req.Email, req.FullName, req.ImageURL)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return user, http.StatusOK, nil
		}))

		gr.Get("/fetch", handleJSON(func(r *http.Request) (any, int, error) {
			users, err := ctrl.GetAllUsers(r.Context())
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return users, http.StatusOK, nil
		}))
	})

	r.Post("/create", handleJSON(func(r *http.Request) (any, int, error) {
		var req types.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, http.StatusBadRequest, err
		}
		user, err := ctrl.CreateUser(r.Context(), req.Username, req.Email, req.FullName, req.ImageURL)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return user, http.StatusOK, nil
	}))

	return r
}
