package inventory

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	items map[string]Item
	err   error
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{items: make(map[string]Item)}
}

func (r *memoryRepository) List(context.Context) ([]Item, error) {
	if r.err != nil {
		return nil, r.err
	}
	items := make([]Item, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, nil
}

func (r *memoryRepository) Get(_ context.Context, sku string) (Item, error) {
	if r.err != nil {
		return Item{}, r.err
	}
	item, ok := r.items[sku]
	if !ok {
		return Item{}, ErrNotFound
	}
	return item, nil
}

func (r *memoryRepository) Create(_ context.Context, item Item) (Item, error) {
	if r.err != nil {
		return Item{}, r.err
	}
	if _, exists := r.items[item.SKU]; exists {
		return Item{}, ErrConflict
	}
	item.UpdatedAt = time.Now()
	r.items[item.SKU] = item
	return item, nil
}

func (r *memoryRepository) Update(_ context.Context, sku string, item Item) (Item, error) {
	if r.err != nil {
		return Item{}, r.err
	}
	if _, exists := r.items[sku]; !exists {
		return Item{}, ErrNotFound
	}
	item.UpdatedAt = time.Now()
	r.items[sku] = item
	return item, nil
}

func (r *memoryRepository) Delete(_ context.Context, sku string) error {
	if r.err != nil {
		return r.err
	}
	if _, exists := r.items[sku]; !exists {
		return ErrNotFound
	}
	delete(r.items, sku)
	return nil
}

func TestServiceCRUD(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository)
	ctx := context.Background()
	item := Item{SKU: "SKU-1", Name: "Widget", Quantity: 4}

	created, err := service.Create(ctx, item)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.SKU != item.SKU || created.Quantity != item.Quantity || created.UpdatedAt.IsZero() {
		t.Fatalf("Create() = %+v, want saved item with timestamp", created)
	}

	items, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].SKU != item.SKU {
		t.Fatalf("List() = %+v, want one item with SKU %q", items, item.SKU)
	}

	item.Name = "Updated widget"
	item.Quantity = 6
	updated, err := service.Update(ctx, item.SKU, item)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != item.Name || updated.Quantity != item.Quantity {
		t.Fatalf("Update() = %+v, want name %q and quantity %d", updated, item.Name, item.Quantity)
	}

	got, err := service.Get(ctx, item.SKU)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != updated {
		t.Fatalf("Get() = %+v, want %+v", got, updated)
	}

	if err := service.Delete(ctx, item.SKU); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(ctx, item.SKU); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestServiceCreateRejectsInvalidItems(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{name: "missing SKU", item: Item{Name: "Widget"}},
		{name: "SKU too long", item: Item{SKU: string(make([]byte, 65)), Name: "Widget"}},
		{name: "missing name", item: Item{SKU: "SKU-1"}},
		{name: "name too long", item: Item{SKU: "SKU-1", Name: string(make([]byte, 201))}},
		{name: "negative quantity", item: Item{SKU: "SKU-1", Name: "Widget", Quantity: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newMemoryRepository()
			_, err := NewService(repository).Create(context.Background(), test.item)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create() error = %v, want ErrInvalid", err)
			}
			if len(repository.items) != 0 {
				t.Fatalf("Create() persisted invalid item: %+v", repository.items)
			}
		})
	}
}

func TestServiceUpdateRejectsMismatchedSKU(t *testing.T) {
	repository := newMemoryRepository()
	_, err := NewService(repository).Update(
		context.Background(),
		"SKU-path",
		Item{SKU: "SKU-body", Name: "Widget", Quantity: 1},
	)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Update() error = %v, want ErrInvalid", err)
	}
	if len(repository.items) != 0 {
		t.Fatalf("Update() persisted mismatched item: %+v", repository.items)
	}
}

func TestServicePropagatesRepositoryErrors(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	service := NewService(&memoryRepository{err: repositoryErr})

	_, err := service.Create(context.Background(), Item{SKU: "SKU-1", Name: "Widget", Quantity: 1})
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("Create() error = %v, want wrapped repository error", err)
	}
}

func TestServiceCreatePropagatesConflict(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository)
	item := Item{SKU: "SKU-1", Name: "Widget", Quantity: 1}
	if _, err := service.Create(context.Background(), item); err != nil {
		t.Fatalf("initial Create() error = %v", err)
	}
	if _, err := service.Create(context.Background(), item); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate Create() error = %v, want ErrConflict", err)
	}
}

func TestServicePreservesContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), contextMarker{}, "request-context")
	repository := &contextRepository{Repository: newMemoryRepository()}

	if _, err := NewService(repository).List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}
}

type contextMarker struct{}

type contextRepository struct {
	Repository
}

func (r *contextRepository) List(ctx context.Context) ([]Item, error) {
	if ctx.Value(contextMarker{}) != "request-context" {
		return nil, errors.New("context was not forwarded")
	}
	return r.Repository.List(ctx)
}
