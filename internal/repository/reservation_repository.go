package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"indico-test-be/internal/model"
)

type ReservationRepository interface {
	Create(ctx context.Context, exec DBExecutor, reservation *model.Reservation) error
	FindByID(ctx context.Context, exec DBExecutor, id string) (*model.Reservation, error)
	FindByIDWithLock(ctx context.Context, exec DBExecutor, id string) (*model.Reservation, error)
	UpdateStatus(ctx context.Context, exec DBExecutor, id string, status model.ReservationStatus, confirmedAt *time.Time) error
	ExpireBatch(ctx context.Context, exec DBExecutor, limit int) (int64, error)
}

type reservationRepository struct {
	db *sql.DB
}

func NewReservationRepository(db *sql.DB) ReservationRepository {
	return &reservationRepository{db: db}
}

func (r *reservationRepository) getExec(exec DBExecutor) DBExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *reservationRepository) Create(ctx context.Context, exec DBExecutor, reservation *model.Reservation) error {
	query := `
		INSERT INTO reservations (id, user_id, item_id, quantity, status, expires_at, confirmed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.getExec(exec).ExecContext(ctx, query,
		reservation.ID,
		reservation.UserID,
		reservation.ItemID,
		reservation.Quantity,
		reservation.Status,
		reservation.ExpiresAt,
		reservation.ConfirmedAt,
		reservation.CreatedAt,
		reservation.UpdatedAt,
	)
	return err
}

func (r *reservationRepository) FindByID(ctx context.Context, exec DBExecutor, id string) (*model.Reservation, error) {
	query := `
		SELECT id, user_id, item_id, quantity, status, expires_at, confirmed_at, created_at, updated_at
		FROM reservations WHERE id = $1
	`
	var res model.Reservation
	var confirmedAt sql.NullTime
	err := r.getExec(exec).QueryRowContext(ctx, query, id).Scan(
		&res.ID, &res.UserID, &res.ItemID, &res.Quantity, &res.Status, &res.ExpiresAt, &confirmedAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrReservationNotFound
		}
		return nil, err
	}
	if confirmedAt.Valid {
		res.ConfirmedAt = &confirmedAt.Time
	}
	return &res, nil
}

func (r *reservationRepository) FindByIDWithLock(ctx context.Context, exec DBExecutor, id string) (*model.Reservation, error) {
	query := `
		SELECT id, user_id, item_id, quantity, status, expires_at, confirmed_at, created_at, updated_at
		FROM reservations WHERE id = $1 FOR UPDATE
	`
	var res model.Reservation
	var confirmedAt sql.NullTime
	err := r.getExec(exec).QueryRowContext(ctx, query, id).Scan(
		&res.ID, &res.UserID, &res.ItemID, &res.Quantity, &res.Status, &res.ExpiresAt, &confirmedAt, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrReservationNotFound
		}
		return nil, err
	}
	if confirmedAt.Valid {
		res.ConfirmedAt = &confirmedAt.Time
	}
	return &res, nil
}

func (r *reservationRepository) UpdateStatus(ctx context.Context, exec DBExecutor, id string, status model.ReservationStatus, confirmedAt *time.Time) error {
	query := `UPDATE reservations SET status = $1, confirmed_at = $2, updated_at = $3 WHERE id = $4`
	res, err := r.getExec(exec).ExecContext(ctx, query, status, confirmedAt, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrReservationNotFound
	}
	return nil
}

func (r *reservationRepository) ExpireBatch(ctx context.Context, exec DBExecutor, limit int) (int64, error) {
	now := time.Now().UTC()
	query := `
		WITH expired_batch AS (
			SELECT id, item_id, quantity 
			FROM reservations 
			WHERE status = 'ACTIVE' AND expires_at <= $1
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		),
		updated_reservations AS (
			UPDATE reservations r
			SET status = 'EXPIRED', updated_at = $1
			FROM expired_batch eb
			WHERE r.id = eb.id
			RETURNING r.id
		)
		UPDATE items i
		SET reserved_stock = i.reserved_stock - eb.total_qty, updated_at = $1
		FROM (
			SELECT item_id, SUM(quantity) as total_qty 
			FROM expired_batch 
			GROUP BY item_id
		) eb
		WHERE i.id = eb.item_id;
	`
	result, err := r.getExec(exec).ExecContext(ctx, query, now, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
