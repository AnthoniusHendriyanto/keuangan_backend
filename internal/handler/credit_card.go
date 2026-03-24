package handler

import (
	"time"

	"keuangan_backend/internal/model"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// In-memory store for now
var mockCreditCards = []model.CreditCard{
	{
		ID:        uuid.MustParse("e429b9f7-0000-0000-0000-000000000000"),
		CardName:  "BCA Everyday",
		CutoffDay: 20,
		DueDay:    5,
		CreatedAt: time.Now(),
	},
}

// ListCreditCards handles GET /v1/credit-cards
func ListCreditCards(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": mockCreditCards,
	})
}

// CreateCreditCard handles POST /v1/credit-cards
func CreateCreditCard(c fiber.Ctx) error {
	var req model.CreateCreditCardRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	newCard := model.CreditCard{
		ID:        uuid.New(),
		CardName:  req.CardName,
		CutoffDay: req.CutoffDay,
		DueDay:    req.DueDay,
		CreatedAt: time.Now(),
	}

	mockCreditCards = append(mockCreditCards, newCard)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": newCard,
	})
}
