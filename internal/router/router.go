package router

import (
	"keuangan_backend/internal/handler"
	"keuangan_backend/internal/middleware"
	"keuangan_backend/internal/pdf"
	"keuangan_backend/internal/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupRoutes configures the Fiber application routes based on architecture.md.
func SetupRoutes(app *fiber.App, pool *pgxpool.Pool) {
	// Repositories
	ccRepo := repository.NewCreditCardRepository(pool)
	txRepo := repository.NewTransactionRepository(pool)

	// Utils
	pdfParser := pdf.NewParser()

	// Handlers
	ccHandler := handler.NewCreditCardHandler(ccRepo)
	txHandler := handler.NewTransactionHandler(txRepo, pdfParser)
	authHandler := handler.NewAuthHandler()

	v1 := app.Group("/v1")

	// Public routes
	v1.Get("/health", handler.HealthCheck(pool))

	// Auth routes (public — no JWT required)
	auth := v1.Group("/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/register", authHandler.Register)

	// Authenticated routes
	v1.Use(middleware.AuthMiddleware)

	// Credit Cards API
	creditCards := v1.Group("/credit-cards")
	creditCards.Get("/", ccHandler.ListCreditCards)
	creditCards.Post("/", ccHandler.CreateCreditCard)

	// Transactions API
	transactions := v1.Group("/transactions")
	transactions.Get("/", txHandler.ListTransactions)
	transactions.Post("/", txHandler.CreateTransaction)
	transactions.Post("/upload-statement", txHandler.UploadStatement)
	transactions.Post("/merge", txHandler.MergeTransaction)
}
