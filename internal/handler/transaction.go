package handler

import (
	"time"

	"keuangan_backend/internal/model"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// In-memory store for now
var mockTransactions = []model.Transaction{
	{
		ID:            uuid.MustParse("a1b2c3d4-0000-0000-0000-000000000000"),
		AmountIDR:     55000,
		TransactionDate: time.Date(2026, 3, 15, 8, 30, 0, 0, time.UTC),
		Description:   "Starbucks Coffee",
		Category:      "Food & Beverage",
		Type:          model.TypeManual,
		Status:        model.StatusPending,
		PaymentMethod: model.PaymentMethodCC,
		CreditCard: &model.CreditCardSummary{
			ID:       uuid.MustParse("e429b9f7-0000-0000-0000-000000000000"),
			CardName: "BCA Everyday",
		},
	},
}

// ListTransactions handles GET /v1/transactions
func ListTransactions(c fiber.Ctx) error {
	// Optional filtering by start_date and end_date can be added here
	return c.JSON(fiber.Map{
		"data": mockTransactions,
	})
}

// CreateTransaction handles POST /v1/transactions
func CreateTransaction(c fiber.Ctx) error {
	var req model.CreateTransactionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	newTx := model.Transaction{
		ID:            uuid.New(),
		AmountIDR:     req.AmountIDR,
		TransactionDate: req.TransactionDate,
		Description:   req.Description,
		Category:      req.Category,
		Type:          model.TypeManual,
		Status:        model.StatusPending, // Defaults to PENDING for manual entries
		PaymentMethod: req.PaymentMethod,
		CreditCardID:  req.CreditCardID,
	}

	mockTransactions = append(mockTransactions, newTx)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": newTx,
	})
}

// UploadStatement handles POST /v1/transactions/upload-statement
func UploadStatement(c fiber.Ctx) error {
	// Stub implementation for now based on architecture.md
	// Return the AI Merge Wizard mock payload
	resp := model.UploadStatementResponse{
		ExtractedTransactions: []model.ExtractedTransaction{
			{
				TempID:          "pdf-1234",
				AmountIDR:       55000,
				TransactionDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				Description:     "SBX SUDIRMAN",
				Category:        "Food & Beverage",
			},
		},
		SuggestedMerges: []model.SuggestedMerge{
			{
				PDFTempID:                   "pdf-1234",
				ExistingManualTransactionID: "a1b2c3d4-0000-0000-0000-000000000000",
				ConfidenceScore:             0.98,
			},
		},
	}

	return c.JSON(resp)
}

// MergeTransaction handles POST /v1/transactions/merge
func MergeTransaction(c fiber.Ctx) error {
	var req model.MergeTransactionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Pseudo logic: update the row locally
	for i, tx := range mockTransactions {
		if tx.ID.String() == req.ExistingManualTransactionID {
			mockTransactions[i].Status = model.StatusReconciled
			mockTransactions[i].Type = model.TypePDFParsed
			return c.JSON(fiber.Map{"data": mockTransactions[i], "status": "reconciled"})
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Transaction not found"})
}
