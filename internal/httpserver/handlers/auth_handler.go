package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	authdomain "disparago/internal/domain/auth"
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

	token, claims, err := h.service.Login(c.UserContext(), req.Username, req.Password)
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
			"user_id":      claims.UserID,
			"company_id":   claims.CompanyID,
			"company_name": claims.CompanyName,
			"token":        token,
			"username":     claims.Username,
			"display_name": claims.DisplayName,
			"role":         claims.Role,
			"expires_at":   claims.ExpiresAt,
		},
	})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	role, _ := c.Locals("auth_role").(authdomain.Role)
	userID, _ := c.Locals("auth_user_id").(int64)
	companyID, _ := c.Locals("auth_company_id").(int64)
	companyName, _ := c.Locals("auth_company_name").(string)
	username, _ := c.Locals("auth_username").(string)
	displayName, _ := c.Locals("auth_display_name").(string)
	expiresAt, _ := c.Locals("auth_expires_at").(string)

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"user_id":      userID,
			"company_id":   companyID,
			"company_name": companyName,
			"username":     username,
			"display_name": displayName,
			"role":         role,
			"expires_at":   expiresAt,
		},
	})
}
