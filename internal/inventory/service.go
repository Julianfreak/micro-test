package inventory

import (
	"context"
	"fmt"
	"strings"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]Item, error) {
	return s.repository.List(ctx)
}

func (s *Service) Get(ctx context.Context, sku string) (Item, error) {
	return s.repository.Get(ctx, sku)
}

func (s *Service) Create(ctx context.Context, item Item) (Item, error) {
	if err := validateItem(item); err != nil {
		return Item{}, err
	}
	return s.repository.Create(ctx, item)
}

func (s *Service) CreateIdempotently(ctx context.Context, key string, requestHash [32]byte, item Item) (Item, error) {
	if err := validateItem(item); err != nil {
		return Item{}, err
	}
	return s.repository.CreateIdempotently(ctx, key, requestHash, item)
}

func (s *Service) Update(ctx context.Context, sku string, item Item) (Item, error) {
	if item.SKU != sku {
		return Item{}, fmt.Errorf("%w: sku in request body must match URL", ErrInvalid)
	}
	if err := validateItem(item); err != nil {
		return Item{}, err
	}
	return s.repository.Update(ctx, sku, item)
}

func (s *Service) Delete(ctx context.Context, sku string) error {
	return s.repository.Delete(ctx, sku)
}

func validateItem(item Item) error {
	if strings.TrimSpace(item.SKU) == "" || len(item.SKU) > 64 {
		return fmt.Errorf("%w: sku is required and must be at most 64 characters", ErrInvalid)
	}
	if strings.TrimSpace(item.Name) == "" || len(item.Name) > 200 {
		return fmt.Errorf("%w: name is required and must be at most 200 characters", ErrInvalid)
	}
	if item.Quantity < 0 {
		return fmt.Errorf("%w: quantity must not be negative", ErrInvalid)
	}
	return nil
}
