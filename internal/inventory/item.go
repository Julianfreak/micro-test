package inventory

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("inventory item not found")
	ErrConflict = errors.New("inventory item already exists")
)

// Item represents one SKU and its current stock level.
type Item struct {
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Quantity  int32     `json:"quantity"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	List(context.Context) ([]Item, error)
	Get(context.Context, string) (Item, error)
	Create(context.Context, Item) (Item, error)
	Update(context.Context, string, Item) (Item, error)
	Delete(context.Context, string) error
}
