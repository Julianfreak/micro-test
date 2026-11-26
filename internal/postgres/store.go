package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Julianfreak/micro-test/internal/inventory"
	"github.com/lib/pq"
)

//go:embed migrations/*.sql
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
	for _, file := range []string{
		"migrations/001_inventory_items.sql",
		"migrations/002_idempotency_requests.sql",
	} {
		migration, err := migrations.ReadFile(file)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("read migration %s: %w", file, err)
		}
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply migration %s: %w", file, err)
		}
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

func (s *Store) CreateIdempotently(ctx context.Context, key string, requestHash [32]byte, item inventory.Item) (inventory.Item, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return inventory.Item{}, fmt.Errorf("begin idempotent create transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM idempotency_requests
		WHERE expires_at <= CURRENT_TIMESTAMP`); err != nil {
		return inventory.Item{}, fmt.Errorf("remove expired idempotency records: %w", err)
	}

	var claimed bool
	err = tx.QueryRowContext(ctx, `
		INSERT INTO idempotency_requests
			(idempotency_key, request_hash, outcome, item, expires_at)
		VALUES ($1, $2, 'pending', NULL, CURRENT_TIMESTAMP + INTERVAL '24 hours')
		ON CONFLICT (idempotency_key) DO UPDATE
		SET request_hash = EXCLUDED.request_hash,
			outcome = 'pending',
			item = NULL,
			created_at = CURRENT_TIMESTAMP,
			expires_at = EXCLUDED.expires_at
		WHERE idempotency_requests.expires_at <= CURRENT_TIMESTAMP
		RETURNING TRUE`,
		key, requestHash[:]).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		var storedHash []byte
		var outcome string
		var storedItem []byte
		err = tx.QueryRowContext(ctx, `
			SELECT request_hash, outcome, item
			FROM idempotency_requests
			WHERE idempotency_key = $1 AND expires_at > CURRENT_TIMESTAMP
			FOR UPDATE`, key).Scan(&storedHash, &outcome, &storedItem)
		if err != nil {
			return inventory.Item{}, fmt.Errorf("load idempotency record: %w", err)
		}
		if !bytes.Equal(storedHash, requestHash[:]) {
			return inventory.Item{}, inventory.ErrIdempotencyKeyReuse
		}

		switch outcome {
		case "created":
			if err := json.Unmarshal(storedItem, &item); err != nil {
				return inventory.Item{}, fmt.Errorf("decode idempotent create response: %w", err)
			}
		case "conflict":
			if err := tx.Commit(); err != nil {
				return inventory.Item{}, fmt.Errorf("commit idempotent create replay: %w", err)
			}
			return inventory.Item{}, inventory.ErrConflict
		default:
			return inventory.Item{}, fmt.Errorf("unexpected idempotency outcome %q", outcome)
		}
		if err := tx.Commit(); err != nil {
			return inventory.Item{}, fmt.Errorf("commit idempotent create replay: %w", err)
		}
		return item, nil
	}
	if err != nil {
		return inventory.Item{}, fmt.Errorf("claim idempotency key: %w", err)
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO inventory_items (sku, name, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (sku) DO NOTHING
		RETURNING sku, name, quantity, updated_at`,
		item.SKU, item.Name, item.Quantity).
		Scan(&item.SKU, &item.Name, &item.Quantity, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		result, err := tx.ExecContext(ctx, `
			UPDATE idempotency_requests
			SET outcome = 'conflict'
			WHERE idempotency_key = $1`, key)
		if err != nil {
			return inventory.Item{}, fmt.Errorf("store idempotent create conflict: %w", err)
		}
		if err := requireOneRow(result, "store idempotent create conflict"); err != nil {
			return inventory.Item{}, err
		}
		if err := tx.Commit(); err != nil {
			return inventory.Item{}, fmt.Errorf("commit idempotent create conflict: %w", err)
		}
		return inventory.Item{}, inventory.ErrConflict
	}
	if err != nil {
		return inventory.Item{}, fmt.Errorf("insert idempotent inventory item: %w", err)
	}

	itemJSON, err := json.Marshal(item)
	if err != nil {
		return inventory.Item{}, fmt.Errorf("encode idempotent create response: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE idempotency_requests
		SET outcome = 'created', item = $2
		WHERE idempotency_key = $1`, key, itemJSON)
	if err != nil {
		return inventory.Item{}, fmt.Errorf("store idempotent create response: %w", err)
	}
	if err := requireOneRow(result, "store idempotent create response"); err != nil {
		return inventory.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return inventory.Item{}, fmt.Errorf("commit idempotent inventory create: %w", err)
	}
	return item, nil
}

func requireOneRow(result sql.Result, operation string) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: check affected rows: %w", operation, err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("%s: affected %d rows, want 1", operation, rowsAffected)
	}
	return nil
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
