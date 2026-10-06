package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"indico-test-be/internal/model"
)

type ItemRepository interface {
	FindAll(ctx context.Context, exec DBExecutor) ([]model.Item, error)
	FindByID(ctx context.Context, exec DBExecutor, id string) (*model.Item, error)
	FindByIDWithLock(ctx context.Context, exec DBExecutor, id string) (*model.Item, error)
	IncrementReservedStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error
	CommitPhysicalStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error
	ReleaseReservedStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error
}

type itemRepository struct {
	db *sql.DB
}

func NewItemRepository(db *sql.DB) ItemRepository {
	return &itemRepository{db: db}
}

func (r *itemRepository) getExec(exec DBExecutor) DBExecutor {
	if exec != nil {
		return exec
	}
	return r.db
}

func (r *itemRepository) FindAll(ctx context.Context, exec DBExecutor) ([]model.Item, error) {
	query := `SELECT id, name, total_stock, reserved_stock, created_at, updated_at FROM items ORDER BY id ASC`
	rows, err := r.getExec(exec).QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var item model.Item
		if err := rows.Scan(&item.ID, &item.Name, &item.TotalStock, &item.ReservedStock, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *itemRepository) FindByID(ctx context.Context, exec DBExecutor, id string) (*model.Item, error) {
	query := `SELECT id, name, total_stock, reserved_stock, created_at, updated_at FROM items WHERE id = $1`
	var item model.Item
	err := r.getExec(exec).QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.Name, &item.TotalStock, &item.ReservedStock, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *itemRepository) FindByIDWithLock(ctx context.Context, exec DBExecutor, id string) (*model.Item, error) {
	query := `SELECT id, name, total_stock, reserved_stock, created_at, updated_at FROM items WHERE id = $1 FOR UPDATE`
	var item model.Item
	err := r.getExec(exec).QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.Name, &item.TotalStock, &item.ReservedStock, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *itemRepository) IncrementReservedStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error {
	query := `UPDATE items SET reserved_stock = reserved_stock + $1, updated_at = $2 WHERE id = $3`
	res, err := r.getExec(exec).ExecContext(ctx, query, quantity, time.Now().UTC(), itemID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrItemNotFound
	}
	return nil
}

func (r *itemRepository) CommitPhysicalStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error {
	query := `UPDATE items SET total_stock = total_stock - $1, reserved_stock = reserved_stock - $1, updated_at = $2 WHERE id = $3`
	res, err := r.getExec(exec).ExecContext(ctx, query, quantity, time.Now().UTC(), itemID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrItemNotFound
	}
	return nil
}

func (r *itemRepository) ReleaseReservedStock(ctx context.Context, exec DBExecutor, itemID string, quantity int) error {
	query := `UPDATE items SET reserved_stock = reserved_stock - $1, updated_at = $2 WHERE id = $3`
	res, err := r.getExec(exec).ExecContext(ctx, query, quantity, time.Now().UTC(), itemID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return model.ErrItemNotFound
	}
	return nil
}
