package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS строит middleware из allowlist origin'ов конфига — не "*": браузер прикладывает
// заголовок Authorization с JWT к запросам с дашборда, а "*" вместе с credentials
// запрещён самой CORS-спецификацией и обесценивает allowlist.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
