package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"keuangan_backend/internal/router"
	"keuangan_backend/internal/security"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Initialize structured logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// 2. Load .env file
	if err := godotenv.Overload(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	// 3. Setup Database Connection Pool
	dbURL := os.Getenv("SUPABASE_DB_URL")
	if dbURL == "" {
		slog.Error("SUPABASE_DB_URL is not set")
		os.Exit(1)
	}

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		slog.Error("Failed to parse database URL", "error", err)
		os.Exit(1)
	}

	// Tweak pool settings for production-readiness
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Ping check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully connected to Supabase PostgreSQL")

	// 4. Initialize Fiber
	app := fiber.New(fiber.Config{
		AppName:   "True Liability Tracker API v1",
		BodyLimit: 10 * 1024 * 1024, // 10MB limit
	})

	// Middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Origin, Content-Type, Accept, Authorization"},
		AllowMethods: []string{"GET, POST, PUT, DELETE, OPTIONS"},
	}))
	app.Use(logger.New())

	// 5. Antivirus Scanner
	var avScanner security.AvScanner
	clamavURL := os.Getenv("CLAMAV_URL")
	if clamavURL != "" {
		avScanner = security.NewClamAVScanner(clamavURL)
		slog.Info("ClamAV scanner initialized", "url", clamavURL)
	} else {
		avScanner = &security.NoopScanner{}
		slog.Info("ClamAV scanner disabled (no CLAMAV_URL set)")
	}

	// Routes
	router.SetupRoutes(app, pool, avScanner)

	// Graceful Shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		slog.Info("Gracefully shutting down...")
		_ = app.Shutdown()
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("Server is starting", "port", port)
	if err := app.Listen(":" + port); err != nil {
		slog.Error("Failed to start server", "error", err)
	}

	slog.Info("Shutdown complete")
}
