package repository

import (
	"context"
	"keuangan_backend/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreditCardRepository struct {
	pool *pgxpool.Pool
}

func NewCreditCardRepository(pool *pgxpool.Pool) *CreditCardRepository {
	return &CreditCardRepository{pool: pool}
}

func (r *CreditCardRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]model.CreditCard, error) {
	query := `SELECT id, user_id, card_name, cutoff_day, due_day, created_at FROM public.credit_cards WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]model.CreditCard, 0)
	for rows.Next() {
		var c model.CreditCard
		if err := rows.Scan(&c.ID, &c.UserID, &c.CardName, &c.CutoffDay, &c.DueDay, &c.CreatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func (r *CreditCardRepository) Create(ctx context.Context, userID uuid.UUID, card model.CreditCard) (model.CreditCard, error) {
	query := `INSERT INTO public.credit_cards (user_id, card_name, cutoff_day, due_day) 
	          VALUES ($1, $2, $3, $4) RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, query, userID, card.CardName, card.CutoffDay, card.DueDay).Scan(&card.ID, &card.CreatedAt)
	if err != nil {
		return model.CreditCard{}, err
	}
	card.UserID = userID
	return card, nil
}
