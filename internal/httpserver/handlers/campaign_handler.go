package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/service"
)

type CampaignHandler struct {
	service *service.CampaignService
}

type CreateCampaignRequest struct {
	Name       string   `json:"name"`
	InstanceID string   `json:"instance_id"`
	Message    string   `json:"message"`
	Contacts   []string `json:"contacts"`
}

func NewCampaignHandler(service *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{service: service}
}

func (h *CampaignHandler) Create(c *fiber.Ctx) error {
	var req CreateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	result, err := h.service.Create(c.UserContext(), service.CreateCampaignInput{
		Name:       req.Name,
		InstanceID: req.InstanceID,
		Message:    req.Message,
		Contacts:   req.Contacts,
	})
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidCampaignInput) {
			status = fiber.StatusBadRequest
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "campaign created",
		"data": fiber.Map{
			"id":             result.ID,
			"name":           result.Name,
			"instance_id":    result.InstanceID,
			"contacts":       result.TotalMessages,
			"status":         result.Status,
			"total_messages": result.TotalMessages,
		},
	})
}

func (h *CampaignHandler) Show(c *fiber.Ctx) error {
	campaign, err := h.service.Get(c.UserContext(), c.Params("id"))
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrCampaignNotFound) {
			status = fiber.StatusNotFound
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": campaign,
	})
}

func (h *CampaignHandler) List(c *fiber.Ctx) error {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	campaigns, err := h.service.List(c.UserContext(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": campaigns,
	})
}

func (h *CampaignHandler) ListMessages(c *fiber.Ctx) error {
	messages, err := h.service.ListMessages(c.UserContext(), c.Params("id"))
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrCampaignNotFound) {
			status = fiber.StatusNotFound
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"campaign_id": c.Params("id"),
			"messages":    messages,
		},
	})
}

func (h *CampaignHandler) Pause(c *fiber.Ctx) error {
	item, err := h.service.Pause(c.UserContext(), c.Params("id"))
	if err != nil {
		status := fiber.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrCampaignNotFound):
			status = fiber.StatusNotFound
		case errors.Is(err, service.ErrInvalidCampaignState):
			status = fiber.StatusConflict
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "campaign paused",
		"data":    item,
	})
}

func (h *CampaignHandler) Resume(c *fiber.Ctx) error {
	item, err := h.service.Resume(c.UserContext(), c.Params("id"))
	if err != nil {
		status := fiber.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrCampaignNotFound):
			status = fiber.StatusNotFound
		case errors.Is(err, service.ErrInvalidCampaignState):
			status = fiber.StatusConflict
		}

		return c.Status(status).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "campaign resumed",
		"data":    item,
	})
}
