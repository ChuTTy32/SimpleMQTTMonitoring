// package main — composition root: читает конфиг, поднимает pgxpool, собирает router,
// стартует HTTP-сервер с graceful shutdown.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/config"
	"github.com/ChuTTy32/SimpleMQTTMonitoring/api/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// ctx отменяется по Ctrl+C/SIGTERM — используется и для pool.Ping на старте,
	// и как родитель для graceful shutdown ниже.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// Fail-fast: если БД недоступна, сервис не должен подниматься и отвечать 500 на
	// каждый запрос — лучше упасть сразу при старте с понятной причиной.
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("pgxpool ping: %v", err)
	}

	router := handler.NewRouter(cfg)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
