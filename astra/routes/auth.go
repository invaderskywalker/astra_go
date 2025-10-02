// astra/routes/auth.go (new)
package routes

import (
	"astra/astra/controllers"
	"astra/astra/types"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func AuthRoutes(ctrl *controllers.AuthController) chi.Router {
	r := chi.NewRouter()
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		token, err := ctrl.Login(r.Context(), req.Username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"token": token})
	})
	return r
}
