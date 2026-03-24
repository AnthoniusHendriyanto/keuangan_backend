package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"keuangan_backend/internal/router"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func main() {
	// Initialize structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	app := fiber.New(fiber.Config{
		AppName: "True Liability Tracker API v1",
	})

	// Middleware
	app.Use(logger.New())

	// Routes
	router.SetupRoutes(app)

	// Graceful Shutdown Setup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		slog.Info("Gracefully shutting down...")
		_ = app.Shutdown()
	}()

	slog.Info("Server is starting on :8080")
	if err := app.Listen(":8080"); err != nil {
		slog.Error("Failed to start server", "error", err)
	}

	slog.Info("Shutdown complete")
}
