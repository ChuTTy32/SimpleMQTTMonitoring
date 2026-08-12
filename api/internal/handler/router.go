// Package handler — HTTP-слой (chi): decode → validate → map → service → respond.
package handler

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/config"
	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/middleware"
)

// Deps — зависимости роутера. Структура, а не список аргументов: модулей будет много
// (sensors, readings, auth, alerts...), и добавление каждого не должно менять сигнатуру
// и ломать все места вызова.
type Deps struct {
	Controllers *ControllerHandler
	Sensors     *SensorHandler
}

// NewRouter собирает chi.Router: глобальный middleware + монтирование модулей.
func NewRouter(cfg *config.Config, deps Deps) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	r.Mount("/controllers", deps.Controllers.Routes())
	r.Mount("/sensors", deps.Sensors.Routes())

	return r
}
