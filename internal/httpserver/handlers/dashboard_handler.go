package handlers

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
)

type DashboardHandler struct {
	distDir string
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{
		distDir: filepath.Join("web", "dist"),
	}
}

func (h *DashboardHandler) Index(c *fiber.Ctx) error {
	indexPath := filepath.Join(h.distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("frontend build not found")
	}

	return c.SendFile(indexPath)
}

func (h *DashboardHandler) DistDir() string {
	return h.distDir
}
