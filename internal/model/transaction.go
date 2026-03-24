package model

import (
	"time"

	"github.com/google/uuid"
)

// Transaction status and types
const (
	TypeManual            = "MANUAL"
	TypePDFParsed         = "PDF_PARSED"
	StatusPending         = "PENDING"
	StatusReconciled      = "RECONCILED"
	PaymentMethodCC       = "CREDIT_CARD"
	PaymentMethodCash     = "CASH"
	PaymentMethodQR       = "QR_BANK"
)

// CreditCardSummary represents embedded minimal card info in transactions.
type CreditCardSummary struct {
	ID       uuid.UUID `json:"id"`
	CardName string    `json:"card_name"`
}

// Transaction represents a financial flow entry.
type Transaction struct {
	ID            uuid.UUID          `json:"id"`
	UserID        uuid.UUID          `json:"user_id,omitempty"`
	AmountIDR     int64              `json:"amount_idr"`
	TransactionDate time.Time        `json:"transaction_date"`
	Description   string             `json:"description"`
	Category      string             `json:"category"`
	Type          string             `json:"type"`
	Status        string             `json:"status"`
	PaymentMethod string             `json:"payment_method"`
	CreditCardID  *uuid.UUID         `json:"credit_card_id,omitempty"`
	CreditCard    *CreditCardSummary `json:"credit_card,omitempty"`
}

// CreateTransactionRequest represents a manual input payload.
type CreateTransactionRequest struct {
	AmountIDR     int64      `json:"amount_idr"`
	TransactionDate time.Time  `json:"transaction_date"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	PaymentMethod string     `json:"payment_method"`
	CreditCardID  *uuid.UUID `json:"credit_card_id"`
}

// UploadStatementResponse represents the merge wizard UI data.
type UploadStatementResponse struct {
	ExtractedTransactions []ExtractedTransaction `json:"extracted_transactions"`
	SuggestedMerges       []SuggestedMerge       `json:"suggested_merges"`
}

// ExtractedTransaction represents parsed PDF data.
type ExtractedTransaction struct {
	TempID          string    `json:"temp_id"`
	AmountIDR       int64     `json:"amount_idr"`
	TransactionDate time.Time `json:"transaction_date"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
}

// SuggestedMerge describes a potential match.
type SuggestedMerge struct {
	PDFTempID                   string  `json:"pdf_temp_id"`
	ExistingManualTransactionID string  `json:"existing_manual_transaction_id"`
	ConfidenceScore             float64 `json:"confidence_score"`
}

// MergeTransactionRequest represents the override payload.
type MergeTransactionRequest struct {
	ExistingManualTransactionID string               `json:"existing_manual_transaction_id"`
	PDFOverrideData             ExtractedTransaction `json:"pdf_override_data"`
}
