package handler

import (
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"keuangan_backend/internal/model"
	"keuangan_backend/internal/pdf"
	"keuangan_backend/internal/repository"
	"keuangan_backend/internal/security"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type TransactionHandler struct {
	repo    *repository.TransactionRepository
	parser  *pdf.Parser
	scanner security.AvScanner
}

func NewTransactionHandler(repo *repository.TransactionRepository, parser *pdf.Parser, scanner security.AvScanner) *TransactionHandler {
	return &TransactionHandler{repo: repo, parser: parser, scanner: scanner}
}

// ListTransactions handles GET /v1/transactions
func (h *TransactionHandler) ListTransactions(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var startDate, endDate *time.Time
	if s := c.Query("start_date"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			startDate = &t
		}
	}
	if e := c.Query("end_date"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			endDate = &t
		}
	}

	transactions, err := h.repo.ListByUserID(c.Context(), userID, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

// CreateTransaction handles POST /v1/transactions
func (h *TransactionHandler) CreateTransaction(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var req model.CreateTransactionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	tx := model.Transaction{
		AmountIDR:       req.AmountIDR,
		TransactionDate: req.TransactionDate,
		Description:     req.Description,
		Category:        req.Category,
		Type:            model.TypeManual,
		Status:          model.StatusPending,
		PaymentMethod:   req.PaymentMethod,
		CreditCardID:    req.CreditCardID,
	}

	newTx, err := h.repo.Create(c.Context(), userID, tx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": newTx,
	})
}

// UploadStatement handles POST /v1/transactions/upload-statement
func (h *TransactionHandler) UploadStatement(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "PDF file is required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer file.Close()
	// Get the optional password from form data
	password := c.FormValue("password")

	// 1. Antivirus Scan
	if err := h.scanner.Scan(c.Context(), file); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Security check failed: %v", err),
		})
	}

	// Reset file pointer after scan
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// 2. Parse PDF
	extracted, err := h.parser.ParseStatement(file, password) // Modified to pass password
	if err != nil {
		if errors.Is(err, pdf.ErrPasswordRequired) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "This PDF is password-protected. Please provide the password in the request.",
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// 2. Fetch existing PENDING manual transactions for fuzzy matching
	existing, err := h.repo.ListByUserID(c.Context(), userID, nil, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch existing transactions"})
	}

	var suggestedMerges []model.SuggestedMerge
	for _, ext := range extracted {
		for _, ex := range existing {
			// Skip if already reconciled
			if ex.Status != model.StatusPending {
				continue
			}

			// Simple fuzzy match: exact amount match and date difference <= 3 days
			dateDiff := math.Abs(ext.TransactionDate.Sub(ex.TransactionDate).Hours()) / 24
			if ext.AmountIDR == ex.AmountIDR && dateDiff <= 3 {
				// Score is higher if days are closer (1.0 for same day, 0.9 for 3 days)
				score := 1.0 - (dateDiff * 0.03) 
				suggestedMerges = append(suggestedMerges, model.SuggestedMerge{
					PDFTempID:                   ext.TempID,
					ExistingManualTransactionID: ex.ID.String(),
					ConfidenceScore:             score,
				})
			}
		}
	}

	return c.JSON(model.UploadStatementResponse{
		ExtractedTransactions: extracted,
		SuggestedMerges:       suggestedMerges,
	})
}

// MergeTransaction handles POST /v1/transactions/merge
func (h *TransactionHandler) MergeTransaction(c fiber.Ctx) error {
	var req model.MergeTransactionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	txID, err := uuid.Parse(req.ExistingManualTransactionID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid transaction ID"})
	}

	err = h.repo.UpdateToReconciled(c.Context(), txID, req.PDFOverrideData.AmountIDR, req.PDFOverrideData.TransactionDate, req.PDFOverrideData.Description)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "reconciled"})
}
