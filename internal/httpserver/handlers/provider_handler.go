package handlers

import (
	"github.com/gofiber/fiber/v2"

	"disparago/internal/evolutiongo"
)

type ProviderHandler struct {
	provider *evolutiongo.Client
}

func NewProviderHandler(provider *evolutiongo.Client) *ProviderHandler {
	return &ProviderHandler{provider: provider}
}

func (h *ProviderHandler) ListEvolutionInstances(c *fiber.Ctx) error {
	instances, err := h.provider.ListInstances(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}

	result := make([]fiber.Map, 0, len(instances))
	for _, item := range instances {
		result = append(result, fiber.Map{
			"id":        item.ID,
			"name":      item.Name,
			"connected": item.Connected,
		})
	}

	return c.JSON(fiber.Map{"data": result})
}

