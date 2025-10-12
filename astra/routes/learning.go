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

func handleLearningJSON(handler func(r *http.Request) (any, int, error)) http.HandlerFunc {
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

func LearningRoutes(ctrl *controllers.LearningController, cfg config.Config) chi.Router {
	r := chi.NewRouter()
	// Auth required
	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))

		gr.Get("/fetch/{user_id}", handleLearningJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			learnings, err := ctrl.GetAllLearningKnowledgeByUser(r.Context(), id)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return learnings, http.StatusOK, nil
		}))

		gr.Get("/fetch/{user_id}/type/{knowledge_type}", handleLearningJSON(func(r *http.Request) (any, int, error) {
			idStr := chi.URLParam(r, "user_id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return nil, http.StatusBadRequest, err
			}
			typeStr := chi.URLParam(r, "knowledge_type")
			learnings, err := ctrl.GetAllLearningKnowledgeByUserAndType(r.Context(), id, typeStr)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return learnings, http.StatusOK, nil
		}))
	})
	return r
}
