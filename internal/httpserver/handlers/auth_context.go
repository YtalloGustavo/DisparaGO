package handlers

import (
	"github.com/gofiber/fiber/v2"

	authdomain "disparago/internal/domain/auth"
)

func actorFromContext(c *fiber.Ctx) authdomain.Actor {
	role, _ := c.Locals("auth_role").(authdomain.Role)
	username, _ := c.Locals("auth_username").(string)
	displayName, _ := c.Locals("auth_display_name").(string)
	companyName, _ := c.Locals("auth_company_name").(string)

	actor := authdomain.Actor{
		Username:    username,
		DisplayName: displayName,
		CompanyName: companyName,
		Role:        role,
	}

	if userID, ok := c.Locals("auth_user_id").(int64); ok {
		actor.UserID = userID
	}
	if companyID, ok := c.Locals("auth_company_id").(int64); ok {
		actor.CompanyID = companyID
	}

	return actor
}
