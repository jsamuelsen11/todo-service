package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"todo-service/internal/db"
	"todo-service/internal/handler"
	"todo-service/internal/logger"
	"todo-service/internal/middleware"
)

func main() {
	// Logger
	logCfg := logger.DefaultConfig()
	log, logCloser := logger.New(logCfg)
	defer logCloser.Close()
	slog.SetDefault(log)

	// Database
	repo, err := db.New("./data/todos.db", log)
	if err != nil {
		log.Error("failed to initialize database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer repo.Close()

	// Router with middleware
	router := chi.NewMux()
	router.Use(chimw.RequestID)
	router.Use(chimw.RealIP)
	router.Use(middleware.RequestLogger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())
	router.Use(chimw.Timeout(30 * time.Second))

	// Health check (plain chi route, outside huma)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Huma API (OpenAPI 3.1)
	config := huma.DefaultConfig("TODO Service API", "1.0.0")
	config.Info.Description = "A local TODO API service with progress tracking."
	api := humachi.New(router, config)

	// Register routes
	todoHandler := handler.NewTodoHandler(repo, log)
	todoHandler.RegisterRoutes(api)

	// Server with graceful shutdown
	addr := ":8080"
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Info("server starting", slog.String("addr", addr), slog.String("docs", "http://localhost:8080/docs"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Info("server stopped")
}
