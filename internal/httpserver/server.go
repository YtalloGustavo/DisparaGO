package httpserver

import (
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"disparago/internal/httpserver/handlers"
	"disparago/internal/httpserver/routes"
	"disparago/internal/service"
)

func New(
	cfg AppConfig,
	authService *service.AuthService,
	campaignService *service.CampaignService,
	instanceSettingsService *service.InstanceSettingsService,
	webhookService *service.WebhookService,
	healthHandler *handlers.HealthHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      cfg.Name,
		BodyLimit:    10 * 1024 * 1024,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	app.Use(recover.New())

	campaignHandler := handlers.NewCampaignHandler(campaignService)
	instanceSettingsHandler := handlers.NewInstanceSettingsHandler(instanceSettingsService)
	authHandler := handlers.NewAuthHandler(authService)
	integrationHandler := handlers.NewIntegrationHandler(authService, campaignService)
	webhookHandler := handlers.NewWebhookHandler(webhookService, instanceSettingsService)
	dashboardHandler := handlers.NewDashboardHandler()

	app.Static("/assets", filepath.Join(dashboardHandler.DistDir(), "assets"))

	routes.Register(app, cfg.InternalAPIKey, authService, healthHandler, authHandler, integrationHandler, campaignHandler, instanceSettingsHandler, webhookHandler, dashboardHandler)

	return app
}

type AppConfig struct {
	Name           string
	PublicURL      string
	InternalAPIKey string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}
