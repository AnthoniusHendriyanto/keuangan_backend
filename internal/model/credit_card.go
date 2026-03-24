package model

import (
	"time"

	"github.com/google/uuid"
)

// CreditCard represents a user's credit card.
type CreditCard struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id,omitempty"` // Omit for now since auth isn't wired yet
	CardName  string    `json:"card_name"`
	CutoffDay int       `json:"cutoff_day"`
	DueDay    int       `json:"due_day"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCreditCardRequest represents the payload for creating a credit card.
type CreateCreditCardRequest struct {
	CardName  string `json:"card_name"`
	CutoffDay int    `json:"cutoff_day"`
	DueDay    int    `json:"due_day"`
}
