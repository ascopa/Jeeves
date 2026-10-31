package repository

import (
	"context"
	"errors"

	"jeeves/checkout-service/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := conn(ctx, r.pool).QueryRow(ctx, `SELECT id, name, email FROM users WHERE id = $1`, userID).
		Scan(&u.ID, &u.Name, &u.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
