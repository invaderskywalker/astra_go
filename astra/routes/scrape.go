package routes

import (
	"encoding/json"
	"net/http"

	"astra/astra/config"
	"astra/astra/controllers"
	"astra/astra/middlewares"
	"astra/astra/utils/types"

	"github.com/go-chi/chi/v5"
)

// ScrapeRoutes registers the scraping-related routes
func ScrapeRoutes(ctrl *controllers.ScrapeController, cfg config.Config) chi.Router {
	r := chi.NewRouter()

	r.Group(func(gr chi.Router) {
		gr.Use(middlewares.AuthMiddleware(cfg))

		// POST /scrape
		gr.Post("/scrape", handleJSON(func(r *http.Request) (any, int, error) {
			var req types.ScrapeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return nil, http.StatusBadRequest, err
			}

			userID := r.Context().Value(middlewares.UserIDKey).(int)
			resp, err := ctrl.Scrape(r.Context(), userID, req)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return resp, http.StatusOK, nil
		}))

		// POST /query/web
		gr.Post("/query/web", handleJSON(func(r *http.Request) (any, int, error) {
			var req types.QueryWebRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				return nil, http.StatusBadRequest, err
			}
			// userID := r.Context().Value(middlewares.UserIDKey).(int)

			scrapes, err := ctrl.QueryWebMulti(req.Queries, req.ResultLimit)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			return scrapes, http.StatusOK, nil
		}))
	})

	return r
}
