package repository

import (
	"context"
	"keuangan_backend/internal/model"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

func (r *TransactionRepository) ListByUserID(ctx context.Context, userID uuid.UUID, startDate, endDate *time.Time) ([]model.Transaction, error) {
	query := `SELECT t.id, t.user_id, t.amount_idr, t.transaction_date, t.description, t.category, t.type, t.status, t.payment_method, t.credit_card_id,
	          c.card_name
	          FROM public.transactions t
	          LEFT JOIN public.credit_cards c ON t.credit_card_id = c.id
	          WHERE t.user_id = $1`
	
	args := []interface{}{userID}
	if startDate != nil {
		query += " AND t.transaction_date >= $2"
		args = append(args, *startDate)
	}
	if endDate != nil {
		if startDate != nil {
			query += " AND t.transaction_date <= $3"
		} else {
			query += " AND t.transaction_date <= $2"
		}
		args = append(args, *endDate)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []model.Transaction
	for rows.Next() {
		var t model.Transaction
		var cardName *string
		if err := rows.Scan(&t.ID, &t.UserID, &t.AmountIDR, &t.TransactionDate, &t.Description, &t.Category, &t.Type, &t.Status, &t.PaymentMethod, &t.CreditCardID, &cardName); err != nil {
			return nil, err
		}
		if t.CreditCardID != nil && cardName != nil {
			t.CreditCard = &model.CreditCardSummary{ID: *t.CreditCardID, CardName: *cardName}
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (r *TransactionRepository) Create(ctx context.Context, userID uuid.UUID, tx model.Transaction) (model.Transaction, error) {
	query := `INSERT INTO public.transactions (user_id, amount_idr, transaction_date, description, category, type, status, payment_method, credit_card_id) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`
	err := r.pool.QueryRow(ctx, query, userID, tx.AmountIDR, tx.TransactionDate, tx.Description, tx.Category, tx.Type, tx.Status, tx.PaymentMethod, tx.CreditCardID).Scan(&tx.ID)
	if err != nil {
		return model.Transaction{}, err
	}
	tx.UserID = userID
	return tx, nil
}

func (r *TransactionRepository) UpdateToReconciled(ctx context.Context, txID uuid.UUID, amount int64, date time.Time, desc string) error {
	query := `UPDATE public.transactions SET status = 'RECONCILED', type = 'PDF_PARSED', amount_idr = $1, transaction_date = $2, description = $3 WHERE id = $4`
	_, err := r.pool.Exec(ctx, query, amount, date, desc, txID)
	return err
}
