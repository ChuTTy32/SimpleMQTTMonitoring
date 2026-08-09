// Package handler — HTTP-слой (chi): decode → validate → map → service → respond.
package handler

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/config"
	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/middleware"
)

// NewRouter собирает chi.Router с глобальным middleware. Маршруты модулей (controllers,
// sensors, ...) монтируются сюда по мере реализации через r.Mount("/controllers", ...) —
// сигнатура уже готова принять их зависимости, когда появится первый repository/service.
func NewRouter(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	return r
}
