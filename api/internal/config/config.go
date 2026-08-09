// Package config — типизированный конфиг сервиса из переменных окружения.
package config

import "github.com/caarlos0/env/v11"

// Config собирает всё, что нужно для старта сервиса, в одном месте: composition root
// (cmd/api/main.go) читает его один раз при старте и раздаёт зависимым частям (pgxpool,
// router, middleware) — сами эти части про переменные окружения не знают.
type Config struct {
	// DBDSN — строка подключения к Postgres/TimescaleDB для pgxpool.
	DBDSN string `env:"DB_DSN,required"`
	// JWTSecret — ключ подписи JWT (middleware/auth.go). required: сервис не должен
	// молча стартовать с пустым секретом и выдавать токены, которые тривиально подделать.
	JWTSecret string `env:"JWT_SECRET,required"`
	// HTTPAddr — адрес, на котором слушает http.Server.
	HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
	// CORSAllowedOrigins — allowlist origin'ов для middleware/cors.go, не "*".
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
}

// Load читает Config из окружения процесса. Ошибку не логирует и не паникует —
// решение, что делать при невалидном конфиге (обычно log.Fatal), остаётся за вызывающим
// кодом в main.go.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
