package app

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/config"
	"disparago/internal/evolutiongo"
	"disparago/internal/httpserver"
	"disparago/internal/httpserver/handlers"
	"disparago/internal/platform/database"
	"disparago/internal/platform/logger"
	"disparago/internal/platform/redisclient"
	"disparago/internal/queue"
	"disparago/internal/repository"
	"disparago/internal/service"
	"disparago/internal/worker"
)

type App struct {
	Config    config.Config
	HTTP      *fiber.App
	DB        *database.Client
	Redis     *redisclient.Client
	Provider  *evolutiongo.Client
	Worker    *worker.DispatchWorker
	Scheduler *worker.CampaignScheduler
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg)

	db, err := database.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	redisClient, err := redisclient.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	provider := evolutiongo.New(cfg)
	authRepository := repository.NewAuthRepository(db)
	campaignRepository := repository.NewCampaignRepository(db)
	instanceSettingsRepository := repository.NewInstanceSettingsRepository(db)
	publisher := queue.NewPublisher(redisClient, cfg.Redis.CampaignMessagesQueue)
	consumer := queue.NewConsumer(redisClient, cfg.Redis.CampaignMessagesQueue)
	authService := service.NewAuthService(authRepository, cfg.Auth)
	if err := authService.EnsureBootstrap(ctx); err != nil {
		return nil, fmt.Errorf("bootstrap auth: %w", err)
	}
	instanceSettingsService := service.NewInstanceSettingsService(instanceSettingsRepository, cfg.App, cfg.Humanizer, cfg.Webhook)
	campaignService := service.NewCampaignService(log, campaignRepository, publisher)
	webhookService := service.NewWebhookService(campaignRepository)
	healthHandler := handlers.NewHealthHandler(cfg, db, redisClient, provider)
	httpApp := httpserver.New(httpserver.AppConfig{
		Name:           cfg.App.Name,
		PublicURL:      cfg.App.PublicURL,
		InternalAPIKey: cfg.InternalAPI.Key,
		ReadTimeout:    cfg.App.ReadTimeout,
		WriteTimeout:   cfg.App.WriteTimeout,
	}, authService, campaignService, instanceSettingsService, webhookService, healthHandler, provider)
	dispatchWorker := worker.NewDispatchWorker(log, campaignRepository, instanceSettingsService, consumer, provider, cfg.Humanizer, cfg.Retry)
	campaignScheduler := worker.NewCampaignScheduler(log, campaignService, cfg.Scheduler)

	return &App{
		Config:    cfg,
		HTTP:      httpApp,
		DB:        db,
		Redis:     redisClient,
		Provider:  provider,
		Worker:    dispatchWorker,
		Scheduler: campaignScheduler,
	}, nil
}

func (a *App) Start() error {
	a.Worker.Start()
	a.Scheduler.Start()

	addr := fmt.Sprintf("%s:%s", a.Config.App.Host, a.Config.App.Port)
	return a.HTTP.Listen(addr)
}

func (a *App) Shutdown(ctx context.Context) error {
	a.Worker.Stop()
	a.Scheduler.Stop()

	if err := a.HTTP.ShutdownWithContext(ctx); err != nil {
		return err
	}

	if err := a.Redis.Close(); err != nil {
		return err
	}

	if err := a.DB.Close(ctx); err != nil {
		return err
	}

	return nil
}
