package handler

import (
	"keuangan_backend/internal/model"
	"keuangan_backend/internal/repository"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type CreditCardHandler struct {
	repo *repository.CreditCardRepository
}

func NewCreditCardHandler(repo *repository.CreditCardRepository) *CreditCardHandler {
	return &CreditCardHandler{repo: repo}
}

// ListCreditCards handles GET /v1/credit-cards
func (h *CreditCardHandler) ListCreditCards(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	cards, err := h.repo.ListByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"data": cards,
	})
}

// CreateCreditCard handles POST /v1/credit-cards
func (h *CreditCardHandler) CreateCreditCard(c fiber.Ctx) error {
	userID := c.Locals("userID").(uuid.UUID)

	var req model.CreateCreditCardRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	card := model.CreditCard{
		CardName:  req.CardName,
		CutoffDay: req.CutoffDay,
		DueDay:    req.DueDay,
	}

	newCard, err := h.repo.Create(c.Context(), userID, card)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": newCard,
	})
}
