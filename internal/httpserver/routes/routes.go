package routes

import (
	"github.com/gofiber/fiber/v2"

	"disparago/internal/httpserver/handlers"
	"disparago/internal/httpserver/middleware"
	"disparago/internal/service"
)

func Register(
	app *fiber.App,
	internalAPIKey string,
	authService *service.AuthService,
	healthHandler *handlers.HealthHandler,
	authHandler *handlers.AuthHandler,
	integrationHandler *handlers.IntegrationHandler,
	campaignHandler *handlers.CampaignHandler,
	instanceSettingsHandler *handlers.InstanceSettingsHandler,
	webhookHandler *handlers.WebhookHandler,
	providerHandler *handlers.ProviderHandler,
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
	api.Post("/webhooks/evolution/:companyID/:instanceID", webhookHandler.EvolutionForInstance)

	internal := app.Group("/api/internal", middleware.InternalKey(internalAPIKey))
	internal.Put("/companies", integrationHandler.UpsertCompany)
	internal.Put("/users", integrationHandler.UpsertUser)
	internal.Post("/campaigns", integrationHandler.CreateOrUpdateCampaign)
	internal.Post("/campaigns/:id/reschedule", integrationHandler.RescheduleCampaign)
	internal.Post("/campaigns/:id/cancel", integrationHandler.CancelCampaign)

	protected := api.Group("/", middleware.Protected(authService))
	protected.Get("/auth/me", authHandler.Me)
	protected.Get("/campaigns", campaignHandler.List)
	protected.Post("/campaigns", campaignHandler.Create)
	protected.Get("/campaigns/:id", campaignHandler.Show)
	protected.Get("/campaigns/:id/messages", campaignHandler.ListMessages)
	protected.Post("/campaigns/:id/pause", campaignHandler.Pause)
	protected.Post("/campaigns/:id/resume", campaignHandler.Resume)
	protected.Post("/campaigns/:id/reschedule", campaignHandler.Reschedule)
	protected.Post("/campaigns/:id/cancel", campaignHandler.CancelScheduled)
	protected.Get("/providers/evolution/instances", providerHandler.ListEvolutionInstances)

	superadmin := protected.Group("/admin", middleware.RequireRole("superadmin"))
	superadmin.Get("/companies/:companyID/instances/settings", instanceSettingsHandler.List)
	superadmin.Get("/companies/:companyID/instances/:instanceID/settings", instanceSettingsHandler.Show)
	superadmin.Put("/companies/:companyID/instances/:instanceID/settings", instanceSettingsHandler.Upsert)

	app.Get("/", dashboardHandler.Index)
	app.Get("/dashboard", dashboardHandler.Index)
}
