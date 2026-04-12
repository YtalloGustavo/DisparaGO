package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/service"
)

type WebhookHandler struct {
	service         *service.WebhookService
	settingsService *service.InstanceSettingsService
}

type EvolutionWebhookRequest struct {
	Event string `json:"event"`
	Data  struct {
		Status     string   `json:"status"`
		MessageIDs []string `json:"messageIds"`
		MessageID  string   `json:"messageId"`
		Key        struct {
			ID string `json:"id"`
		} `json:"key"`
	} `json:"data"`
}

func NewWebhookHandler(service *service.WebhookService, settingsService *service.InstanceSettingsService) *WebhookHandler {
	return &WebhookHandler{
		service:         service,
		settingsService: settingsService,
	}
}

func (h *WebhookHandler) Evolution(c *fiber.Ctx) error {
	return h.processEvolution(c, "", "")
}

func (h *WebhookHandler) EvolutionForInstance(c *fiber.Ctx) error {
	return h.processEvolution(c, c.Params("companyID"), c.Params("instanceID"))
}

func (h *WebhookHandler) processEvolution(c *fiber.Ctx, companyRaw string, instanceID string) error {
	if instanceID != "" {
		companyID, err := strconv.ParseInt(companyRaw, 10, 64)
		if err != nil || companyID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid company id",
			})
		}

		token := c.Query("token")
		if token == "" {
			token = c.Get("X-DisparaGO-Webhook-Token")
		}

		if err := h.settingsService.ValidateWebhookToken(c.UserContext(), companyID, instanceID, token); err != nil {
			status := fiber.StatusUnauthorized
			if errors.Is(err, service.ErrInvalidInstanceSettings) {
				status = fiber.StatusForbidden
			}
			return c.Status(status).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	var req EvolutionWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	messageIDs := req.Data.MessageIDs
	if len(messageIDs) == 0 {
		switch {
		case req.Data.MessageID != "":
			messageIDs = []string{req.Data.MessageID}
		case req.Data.Key.ID != "":
			messageIDs = []string{req.Data.Key.ID}
		}
	}

	result, err := h.service.Track(c.UserContext(), service.EvolutionWebhookInput{
		Event:      req.Event,
		Status:     req.Data.Status,
		MessageIDs: messageIDs,
	})
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrUnsupportedWebhookEvent) {
			status = fiber.StatusAccepted
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "webhook processed",
		"data":    result,
	})
}
