package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/bitEngine-AI/bitengine/internal/auth"
	"github.com/bitEngine-AI/bitengine/internal/setup"
)

func NewRouter(db *sqlx.DB, rdb *redis.Client, jwtSecret string) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	sys := SystemHandler{DB: db, RDB: rdb}
	authH := AuthHandler{DB: db, JWTSecret: jwtSecret}
	setupH := SetupHandler{Wizard: &setup.Wizard{DB: db}}

	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/system/status", sys.Status)
		r.Get("/setup/status", setupH.Status)
		r.Post("/setup/step/1", setupH.Step1)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Protected endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(jwtSecret))
			// Future protected routes go here (apps, ai, etc.)
		})
	})

	return r
}
