package http

import (
	"github.com/Baldyozh/log-processor/internal/delivery/http/handlers"
	"github.com/Baldyozh/log-processor/internal/infrastructure/auth"
	"github.com/Baldyozh/log-processor/internal/infrastructure/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Router sets up the HTTP router with all routes
func Router(
	authHandler *handlers.AuthHandler,
	logHandler *handlers.LogHandler,
	jwtService *auth.JWTService,
	authRepo *postgres.AuthRepository,
) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.AllowContentType("application/json"))
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	// Public routes
	r.Post("/api/auth/login", authHandler.Login)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwtService.AuthMiddleware)

		// Auth routes
		r.Get("/api/auth/me", authHandler.GetCurrentUser)
		r.Get("/api/auth/roles", authHandler.GetAllRoles)

		// Log routes
		r.Route("/api/logs", func(r chi.Router) {
			r.Get("/", logHandler.QueryLogs)
			r.Get("/stats", logHandler.GetLogsStats)
			r.Get("/export", logHandler.ExportLogsToCSV)
			r.Get("/request/{request_id}", logHandler.GetLogsByRequestID)
			r.Get("/{id}", logHandler.GetLogByID)
		})

		// Admin routes
		r.Route("/api/admin", func(r chi.Router) {
			r.Use(auth.RequirePermission(authRepo, "users:manage"))
			r.Post("/users", authHandler.CreateUser)
		})
	})

	return r
}
