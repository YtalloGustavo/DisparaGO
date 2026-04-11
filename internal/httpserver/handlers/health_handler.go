package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/config"
	"disparago/internal/evolutiongo"
	"disparago/internal/platform/database"
	"disparago/internal/platform/redisclient"
)

type HealthHandler struct {
	config   config.Config
	db       *database.Client
	redis    *redisclient.Client
	provider *evolutiongo.Client
}

func NewHealthHandler(
	cfg config.Config,
	db *database.Client,
	redis *redisclient.Client,
	provider *evolutiongo.Client,
) *HealthHandler {
	return &HealthHandler{
		config:   cfg,
		db:       db,
		redis:    redis,
		provider: provider,
	}
}

func (h *HealthHandler) Check(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()

	postgresStatus := "up"
	if err := h.db.Pool.Ping(ctx); err != nil {
		postgresStatus = "down"
	}

	redisStatus := "up"
	if err := h.redis.Redis.Ping(ctx).Err(); err != nil {
		redisStatus = "down"
	}

	statusCode := fiber.StatusOK
	if postgresStatus != "up" || redisStatus != "up" {
		statusCode = fiber.StatusServiceUnavailable
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"service": h.config.App.Name,
		"env":     h.config.App.Env,
		"status":  mapStatus(postgresStatus, redisStatus),
		"dependencies": fiber.Map{
			"postgres":    postgresStatus,
			"redis":       redisStatus,
			"evolutiongo": dependencyStatus(h.provider.BaseURL()),
		},
	})
}

func mapStatus(postgresStatus, redisStatus string) string {
	if postgresStatus == "up" && redisStatus == "up" {
		return "ok"
	}

	return "degraded"
}

func dependencyStatus(baseURL string) string {
	if baseURL == "" {
		return "not_configured"
	}

	return "configured"
}
