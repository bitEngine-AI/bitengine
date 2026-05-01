package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/bitEngine-AI/bitengine/internal/ai"
	"github.com/bitEngine-AI/bitengine/internal/apps"
	"github.com/bitEngine-AI/bitengine/internal/auth"
	"github.com/bitEngine-AI/bitengine/internal/setup"
)

func NewRouter(db *sqlx.DB, rdb *redis.Client, jwtSecret string, ollama *ai.OllamaClient, codegen ai.CodeGen, gen *apps.AppGenerator, svc *apps.AppService, tplSvc *apps.TemplateService, hwInfo *ai.HardwareInfo, models *ai.ModelConfig) chi.Router {
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

	sys := SystemHandler{DB: db, RDB: rdb, CodegenMode: codegen.Mode(), Hardware: hwInfo, Models: models}
	authH := AuthHandler{DB: db, JWTSecret: jwtSecret}
	setupH := SetupHandler{Wizard: &setup.Wizard{DB: db}}
	aiH := AIHandler{
		Ollama:   ollama,
		Intent:   ai.NewIntentEngine(ollama, models.IntentModel),
		CodeGen:  codegen,
		Reviewer: ai.NewCodeReviewer(ollama, models.ReviewModel),
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/system/status", sys.Status)
		r.Get("/system/metrics", sys.Metrics)
		r.Get("/setup/status", setupH.Status)
		r.Post("/setup/step/1", setupH.Step1)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Protected endpoints
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(jwtSecret))
			r.Get("/ai/models", aiH.Models)
			r.Post("/ai/intent", aiH.AnalyzeIntent)
			r.Post("/ai/generate", aiH.GenerateCode)

			appsH := AppsHandler{Generator: gen, Service: svc, Templates: tplSvc}
			r.Post("/apps", appsH.Create)
			r.Get("/apps", appsH.List)
			r.Get("/apps/{id}", appsH.Get)
			r.Delete("/apps/{id}", appsH.Delete)
			r.Post("/apps/{id}/regenerate", appsH.Regenerate)
			r.Post("/apps/{id}/start", appsH.Start)
			r.Post("/apps/{id}/stop", appsH.Stop)
			r.Get("/apps/{id}/logs", appsH.Logs)
			r.Get("/apps/templates", appsH.ListTemplates)
			r.Post("/apps/templates/{slug}/deploy", appsH.DeployTemplate)
		})
	})

	return r
}
