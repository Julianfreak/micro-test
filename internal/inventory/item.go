package inventory

import (
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("inventory item not found")
	ErrConflict            = errors.New("inventory item already exists")
	ErrInvalid             = errors.New("invalid inventory item")
	ErrIdempotencyKeyReuse = errors.New("idempotency key was already used with a different request")
)

// Item represents one SKU and its current stock level.
type Item struct {
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Quantity  int32     `json:"quantity"`
	UpdatedAt time.Time `json:"updated_at"`
}
