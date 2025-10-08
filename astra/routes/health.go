package routes

import (
	"astra/astra/controllers"
	"github.com/go-chi/chi/v5"
)

func HealthRoutes(ctrl *controllers.HealthController) chi.Router {
	r := chi.NewRouter()
	r.Get("/", ctrl.HealthCheck)
	return r
}
