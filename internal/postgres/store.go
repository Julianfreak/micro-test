package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/Julianfreak/micro-test/internal/inventory"
	"github.com/lib/pq"
)

//go:embed migrations/001_inventory_items.sql
var migrations embed.FS

type Store struct {
	db *sql.DB
}

var _ inventory.Repository = (*Store)(nil)

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	migration, err := migrations.ReadFile("migrations/001_inventory_items.sql")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("read inventory migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply inventory migration: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) List(ctx context.Context) ([]inventory.Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sku, name, quantity, updated_at
		FROM inventory_items
		ORDER BY sku`)
	if err != nil {
		return nil, fmt.Errorf("query inventory items: %w", err)
	}
	defer rows.Close()

	items := make([]inventory.Item, 0)
	for rows.Next() {
		var item inventory.Item
		if err := rows.Scan(&item.SKU, &item.Name, &item.Quantity, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan inventory item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory items: %w", err)
	}
	return items, nil
}

func (s *Store) Get(ctx context.Context, sku string) (inventory.Item, error) {
	var item inventory.Item
	err := s.db.QueryRowContext(ctx, `
		SELECT sku, name, quantity, updated_at
		FROM inventory_items WHERE sku = $1`, sku).
		Scan(&item.SKU, &item.Name, &item.Quantity, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return inventory.Item{}, inventory.ErrNotFound
	}
	if err != nil {
		return inventory.Item{}, fmt.Errorf("query inventory item: %w", err)
	}
	return item, nil
}

func (s *Store) Create(ctx context.Context, item inventory.Item) (inventory.Item, error) {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO inventory_items (sku, name, quantity)
		VALUES ($1, $2, $3)
		RETURNING sku, name, quantity, updated_at`,
		item.SKU, item.Name, item.Quantity).
		Scan(&item.SKU, &item.Name, &item.Quantity, &item.UpdatedAt)
	if isUniqueViolation(err) {
		return inventory.Item{}, inventory.ErrConflict
	}
	if err != nil {
		return inventory.Item{}, fmt.Errorf("insert inventory item: %w", err)
	}
	return item, nil
}

func (s *Store) Update(ctx context.Context, sku string, item inventory.Item) (inventory.Item, error) {
	err := s.db.QueryRowContext(ctx, `
		UPDATE inventory_items
		SET name = $2, quantity = $3, updated_at = CURRENT_TIMESTAMP
		WHERE sku = $1
		RETURNING sku, name, quantity, updated_at`,
		sku, item.Name, item.Quantity).
		Scan(&item.SKU, &item.Name, &item.Quantity, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return inventory.Item{}, inventory.ErrNotFound
	}
	if err != nil {
		return inventory.Item{}, fmt.Errorf("update inventory item: %w", err)
	}
	return item, nil
}

func (s *Store) Delete(ctx context.Context, sku string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM inventory_items WHERE sku = $1`, sku)
	if err != nil {
		return fmt.Errorf("delete inventory item: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted inventory item: %w", err)
	}
	if rowsAffected == 0 {
		return inventory.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var postgresErr *pq.Error
	return errors.As(err, &postgresErr) && postgresErr.Code == "23505"
}
