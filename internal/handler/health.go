package handler

import (
	"github.com/gofiber/fiber/v3"
)

// HealthCheck responds with the server status.
func HealthCheck(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
	})
}
