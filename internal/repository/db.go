package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"indico-test-be/internal/config"

	_ "github.com/lib/pq"
)

type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Transactor interface {
	ExecTx(ctx context.Context, fn func(tx *sql.Tx) error) error
	GetDB() *sql.DB
}

type DB struct {
	db *sql.DB
}

func NewDB(cfg *config.Config) (*DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	log.Println("Database connection established successfully.")
	return &DB{db: db}, nil
}

func NewDBFromSQLDB(db *sql.DB) *DB {
	return &DB{db: db}
}

func (d *DB) GetDB() *sql.DB {
	return d.db
}

func (d *DB) ExecTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
