package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	token, claims, err := h.service.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "login successful",
		"data": fiber.Map{
			"token":      token,
			"username":   claims.Username,
			"role":       claims.Role,
			"expires_at": claims.ExpiresAt,
		},
	})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	username, _ := c.Locals("auth_username").(string)
	role, _ := c.Locals("auth_role").(string)
	expiresAt, _ := c.Locals("auth_expires_at").(string)

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"username":   username,
			"role":       role,
			"expires_at": expiresAt,
		},
	})
}
