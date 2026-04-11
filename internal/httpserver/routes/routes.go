package routes

import (
	"github.com/gofiber/fiber/v2"

	"disparago/internal/httpserver/handlers"
	"disparago/internal/httpserver/middleware"
	"disparago/internal/service"
)

func Register(
	app *fiber.App,
	authService *service.AuthService,
	healthHandler *handlers.HealthHandler,
	authHandler *handlers.AuthHandler,
	campaignHandler *handlers.CampaignHandler,
	instanceSettingsHandler *handlers.InstanceSettingsHandler,
	webhookHandler *handlers.WebhookHandler,
	dashboardHandler *handlers.DashboardHandler,
) {
	app.Get("/health", healthHandler.Check)

	api := app.Group("/api/v1")
	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "DisparaGO",
			"status":  "ok",
		})
	})

	api.Post("/auth/login", authHandler.Login)
	api.Post("/webhooks/evolution", webhookHandler.Evolution)
	api.Post("/webhooks/evolution/:instanceID", webhookHandler.EvolutionForInstance)

	protected := api.Group("/", middleware.Protected(authService))
	protected.Get("/auth/me", authHandler.Me)
	protected.Get("/campaigns", campaignHandler.List)
	protected.Post("/campaigns", campaignHandler.Create)
	protected.Get("/campaigns/:id", campaignHandler.Show)
	protected.Get("/campaigns/:id/messages", campaignHandler.ListMessages)
	protected.Post("/campaigns/:id/pause", campaignHandler.Pause)
	protected.Post("/campaigns/:id/resume", campaignHandler.Resume)

	superadmin := protected.Group("/admin", middleware.RequireRole("superadmin"))
	superadmin.Get("/instances/settings", instanceSettingsHandler.List)
	superadmin.Get("/instances/:instanceID/settings", instanceSettingsHandler.Show)
	superadmin.Put("/instances/:instanceID/settings", instanceSettingsHandler.Upsert)

	app.Get("/", dashboardHandler.Index)
	app.Get("/dashboard", dashboardHandler.Index)
}
