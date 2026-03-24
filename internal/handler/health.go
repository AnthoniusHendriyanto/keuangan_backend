package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthCheck responds with the server status and optional DB ping.
func HealthCheck(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		dbStatus := "connected"
		if err := pool.Ping(c.Context()); err != nil {
			dbStatus = "disconnected"
		}
		
		return c.JSON(fiber.Map{
			"status": "ok",
			"database": dbStatus,
		})
	}
}
