// Package middleware содержит сквозную HTTP-логику (логирование, CORS, позже — auth/RBAC),
// применяемую ко всем маршрутам через chi.Router.Use, а не к каждой ручке по отдельности.
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder оборачивает http.ResponseWriter, чтобы подсмотреть статус-код ответа —
// сам http.ResponseWriter его не отдаёт, только принимает через WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Logging логирует каждый запрос после его завершения: метод, путь, статус, длительность.
// Пишет после next.ServeHTTP, а не до — иначе статус и длительность ещё не известны.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Дефолт 200: если хендлер ни разу не вызвал WriteHeader явно (пишет сразу
		// в тело через w.Write), net/http сам подставляет 200 — recorder должен
		// показывать то же самое, а не 0.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		)
	})
}
