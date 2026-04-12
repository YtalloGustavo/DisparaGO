package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	instancecfg "disparago/internal/domain/instance"
	"disparago/internal/service"
)

type InstanceSettingsHandler struct {
	service *service.InstanceSettingsService
}

func NewInstanceSettingsHandler(service *service.InstanceSettingsService) *InstanceSettingsHandler {
	return &InstanceSettingsHandler{service: service}
}

func (h *InstanceSettingsHandler) List(c *fiber.Ctx) error {
	companyID, err := strconv.ParseInt(c.Params("companyID"), 10, 64)
	if err != nil || companyID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid company id"})
	}

	items, err := h.service.ListByCompany(c.UserContext(), companyID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": items})
}

func (h *InstanceSettingsHandler) Show(c *fiber.Ctx) error {
	companyID, parseErr := strconv.ParseInt(c.Params("companyID"), 10, 64)
	if parseErr != nil || companyID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid company id"})
	}

	item, err := h.service.Get(c.UserContext(), companyID, c.Params("instanceID"))
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidInstanceSettings) {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": item})
}

func (h *InstanceSettingsHandler) Upsert(c *fiber.Ctx) error {
	var req instancecfg.Settings
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	companyID, parseErr := strconv.ParseInt(c.Params("companyID"), 10, 64)
	if parseErr != nil || companyID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid company id"})
	}

	req.CompanyID = companyID
	req.InstanceID = c.Params("instanceID")

	item, err := h.service.Save(c.UserContext(), req)
	if err != nil {
		status := fiber.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidInstanceSettings) {
			status = fiber.StatusBadRequest
		}
		return c.Status(status).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "instance settings saved",
		"data":    item,
	})
}
