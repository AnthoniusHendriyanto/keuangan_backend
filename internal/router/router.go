package router

import (
	"keuangan_backend/internal/handler"

	"github.com/gofiber/fiber/v3"
)

// SetupRoutes configures the Fiber application routes based on architecture.md
func SetupRoutes(app *fiber.App) {
	v1 := app.Group("/v1")

	// Health check
	v1.Get("/health", handler.HealthCheck)

	// Credit Cards API
	creditCards := v1.Group("/credit-cards")
	creditCards.Get("/", handler.ListCreditCards)
	creditCards.Post("/", handler.CreateCreditCard)

	// Transactions API
	transactions := v1.Group("/transactions")
	transactions.Get("/", handler.ListTransactions)
	transactions.Post("/", handler.CreateTransaction)
	transactions.Post("/upload-statement", handler.UploadStatement)
	transactions.Post("/merge", handler.MergeTransaction)
}
