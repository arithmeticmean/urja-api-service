// Package routes lists every endpoint in one place.
package routes

import (
	"net/http"

	"urja-api-service/controllers"
	"urja-api-service/middleware"
)

func New(meters *controllers.MeterController, docs *controllers.Docs) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", controllers.Health)
	mux.HandleFunc("GET /openapi.json", docs.Spec)
	mux.HandleFunc("GET /docs", docs.UI)
	mux.HandleFunc("GET /v1/meters", meters.List)
	mux.HandleFunc("GET /v1/meters/{id}", meters.Get)
	mux.HandleFunc("GET /v1/meters/{id}/readings", meters.Readings)
	return middleware.Logger(mux)
}
