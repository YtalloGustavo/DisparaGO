package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func InternalKey(expected string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.TrimSpace(expected) == "" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "internal api disabled"})
		}

		received := strings.TrimSpace(c.Get("X-Internal-Key"))
		if subtle.ConstantTimeCompare([]byte(received), []byte(expected)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid internal key"})
		}

		return c.Next()
	}
}
