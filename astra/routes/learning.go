// astra/routes/long_term.go
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

func handleLongTermJSON(handler func(r *http.Request) (any, int, error)) http.HandlerFunc {
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

func LongTermRoutes(ctrl *controllers.LongTermController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))

		gr.Get("/fetch/{user_id}", handleLongTermJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			longterms, err := ctrl.GetAllLongTermKnowledgeByUser(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return longterms, http.StatusOK, nil
		}))

		gr.Get("/fetch/{user_id}/type/{knowledge_type}", handleLongTermJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			typeStr := chi.URLParam(r, "knowledge_type")
			longterms, err := ctrl.GetAllLongTermKnowledgeByUserAndType(r.Context(), id, typeStr)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return longterms, http.StatusOK, nil
		}))
	})
	return r
}
