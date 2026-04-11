package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"disparago/internal/service"
)

func Protected(authService *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		if header == "" {
			return unauthorized(c)
		}

		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" || token == header {
			return unauthorized(c)
		}

		claims, err := authService.Validate(token)
		if err != nil {
			return unauthorized(c)
		}

		c.Locals("auth_username", claims.Username)
		c.Locals("auth_role", claims.Role)
		c.Locals("auth_expires_at", claims.ExpiresAt.Format(timeLayout))
		return c.Next()
	}
}

func RequireRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		currentRole, _ := c.Locals("auth_role").(string)
		if currentRole != role {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "forbidden",
			})
		}

		return c.Next()
	}
}

const timeLayout = "2006-01-02T15:04:05Z07:00"

func unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": "authentication required",
	})
}
