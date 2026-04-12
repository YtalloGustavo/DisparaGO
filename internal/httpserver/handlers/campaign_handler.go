package handlers

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/service"
)

type CampaignHandler struct {
	service *service.CampaignService
}

type CreateCampaignRequest struct {
	Name             string   `json:"name"`
	InstanceID       string   `json:"instance_id"`
	Message          string   `json:"message"`
	Contacts         []string `json:"contacts"`
	SendMode         string   `json:"send_mode"`
	ScheduledAt      string   `json:"scheduled_at"`
	Timezone         string   `json:"timezone"`
	ExternalSource   string   `json:"external_source"`
	ExternalSourceID string   `json:"external_source_id"`
}

type RescheduleCampaignRequest struct {
	ScheduledAt string `json:"scheduled_at"`
	Timezone    string `json:"timezone"`
}

func NewCampaignHandler(service *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{service: service}
}

func (h *CampaignHandler) Create(c *fiber.Ctx) error {
	var req CreateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	actor := actorFromContext(c)
	result, err := h.service.Create(c.UserContext(), service.CreateCampaignInput{
		CompanyID:        actor.CompanyID,
		CreatedByUserID:  &actor.UserID,
		Name:             req.Name,
		InstanceID:       req.InstanceID,
		Message:          req.Message,
		Contacts:         req.Contacts,
		SendMode:         req.SendMode,
		ScheduledAt:      req.ScheduledAt,
		Timezone:         req.Timezone,
		ExternalSource:   req.ExternalSource,
		ExternalSourceID: req.ExternalSourceID,
	})
	if err != nil {
		return campaignError(c, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "campaign created",
		"data": fiber.Map{
			"id":             result.ID,
			"name":           result.Name,
			"instance_id":    result.InstanceID,
			"contacts":       result.TotalMessages,
			"status":         result.Status,
			"send_mode":      result.SendMode,
			"total_messages": result.TotalMessages,
		},
	})
}

func (h *CampaignHandler) Show(c *fiber.Ctx) error {
	item, err := h.service.Get(c.UserContext(), actorFromContext(c), c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}
	return c.JSON(fiber.Map{"data": item})
}

func (h *CampaignHandler) List(c *fiber.Ctx) error {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	statuses := make([]string, 0)
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value != "" {
				statuses = append(statuses, value)
			}
		}
	}

	items, err := h.service.List(c.UserContext(), actorFromContext(c), service.ListCampaignsInput{
		Statuses: statuses,
		Limit:    limit,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": items})
}

func (h *CampaignHandler) ListMessages(c *fiber.Ctx) error {
	items, err := h.service.ListMessages(c.UserContext(), actorFromContext(c), c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"campaign_id": c.Params("id"),
			"messages":    items,
		},
	})
}

func (h *CampaignHandler) Pause(c *fiber.Ctx) error {
	item, err := h.service.Pause(c.UserContext(), actorFromContext(c), c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "campaign paused",
		"data":    item,
	})
}

func (h *CampaignHandler) Resume(c *fiber.Ctx) error {
	item, err := h.service.Resume(c.UserContext(), actorFromContext(c), c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "campaign resumed",
		"data":    item,
	})
}

func (h *CampaignHandler) Reschedule(c *fiber.Ctx) error {
	var req RescheduleCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	item, err := h.service.Reschedule(c.UserContext(), actorFromContext(c), service.UpdateScheduleInput{
		CampaignID:  c.Params("id"),
		ScheduledAt: req.ScheduledAt,
		Timezone:    req.Timezone,
	})
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "campaign rescheduled",
		"data":    item,
	})
}

func (h *CampaignHandler) CancelScheduled(c *fiber.Ctx) error {
	item, err := h.service.CancelScheduled(c.UserContext(), actorFromContext(c), c.Params("id"))
	if err != nil {
		return campaignError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "campaign canceled",
		"data":    item,
	})
}

func campaignError(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrInvalidCampaignInput):
		status = fiber.StatusBadRequest
	case errors.Is(err, service.ErrCampaignNotFound):
		status = fiber.StatusNotFound
	case errors.Is(err, service.ErrInvalidCampaignState):
		status = fiber.StatusConflict
	}
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}
