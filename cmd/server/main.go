package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/DevenWen/TodoDemo/migrate"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/DevenWen/TodoDemo/internal/config"
	"github.com/DevenWen/TodoDemo/internal/database"
	"github.com/DevenWen/TodoDemo/internal/handler"
	"github.com/DevenWen/TodoDemo/internal/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	logger.Info("connected to database")

	// Run auto migrations
	if err := migrate.AutoMigrate(ctx, database.Pool); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	logger.Info("migrations complete")

	// Set up router
	r := chi.NewRouter()

	// Middleware stack
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL, "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Create handlers
	authHandler := handler.NewAuthHandler(cfg)
	todoHandler := handler.NewTodoHandler(cfg)

	// Auth routes (public)
	r.Get("/auth/github/login", authHandler.GitHubLogin)
	r.Get("/auth/github/callback", authHandler.GitHubCallback)

	// Protected API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(cfg))

		r.Get("/auth/me", authHandler.Me)
		r.Post("/auth/logout", authHandler.Logout)

		r.Get("/todos", todoHandler.List)
		r.Post("/todos", todoHandler.Create)
		r.Put("/todos/{id}", todoHandler.Update)
		r.Delete("/todos/{id}", todoHandler.Delete)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Static file serving with SPA fallback
	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "dist"))

	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		f, err := filesDir.Open(path)
		if err != nil {
			// SPA fallback: serve index.html for client-side routing
			indexFile, err := filesDir.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()
			stat, err := indexFile.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}
			http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile)
			return
		}
		f.Close()
		http.FileServer(filesDir).ServeHTTP(w, r)
	}))

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	logger.Info("server starting", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
