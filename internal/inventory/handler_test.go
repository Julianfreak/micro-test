package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	items map[string]Item
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: make(map[string]Item)}
}

func (s *fakeStore) List(context.Context) ([]Item, error) {
	items := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}

func (s *fakeStore) Get(_ context.Context, sku string) (Item, error) {
	item, ok := s.items[sku]
	if !ok {
		return Item{}, ErrNotFound
	}
	return item, nil
}

func (s *fakeStore) Create(_ context.Context, item Item) (Item, error) {
	if _, ok := s.items[item.SKU]; ok {
		return Item{}, ErrConflict
	}
	item.UpdatedAt = time.Now()
	s.items[item.SKU] = item
	return item, nil
}

func (s *fakeStore) Update(_ context.Context, sku string, item Item) (Item, error) {
	if _, ok := s.items[sku]; !ok {
		return Item{}, ErrNotFound
	}
	item.UpdatedAt = time.Now()
	s.items[sku] = item
	return item, nil
}

func (s *fakeStore) Delete(_ context.Context, sku string) error {
	if _, ok := s.items[sku]; !ok {
		return ErrNotFound
	}
	delete(s.items, sku)
	return nil
}

func TestCreateAndGetInventoryItem(t *testing.T) {
	handler := NewHandler(newFakeStore()).Routes()
	body := `{"sku":"SKU-1","name":"Widget","quantity":4}`

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("POST /items status = %d, want %d; body: %s", create.Code, http.StatusCreated, create.Body)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/items/SKU-1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("GET /items/SKU-1 status = %d, want %d; body: %s", get.Code, http.StatusOK, get.Body)
	}
	var item Item
	if err := json.Unmarshal(get.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item response: %v", err)
	}
	if item.SKU != "SKU-1" || item.Quantity != 4 {
		t.Fatalf("unexpected item response: %+v", item)
	}
}

func TestCreateRejectsInvalidItemsAndDuplicateSKUs(t *testing.T) {
	handler := NewHandler(newFakeStore()).Routes()
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "negative quantity", body: `{"sku":"SKU-1","name":"Widget","quantity":-1}`, want: http.StatusBadRequest},
		{name: "quantity overflow", body: `{"sku":"SKU-1","name":"Widget","quantity":2147483648}`, want: http.StatusBadRequest},
		{name: "unknown field", body: `{"sku":"SKU-1","name":"Widget","quantity":1,"extra":true}`, want: http.StatusBadRequest},
		{name: "duplicate SKU", body: `{"sku":"SKU-1","name":"Widget","quantity":1}`, want: http.StatusCreated},
		{name: "duplicate SKU conflict", body: `{"sku":"SKU-1","name":"Other","quantity":2}`, want: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(test.body)))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestUpdateRequiresMatchingSKU(t *testing.T) {
	handler := NewHandler(newFakeStore()).Routes()
	body := `{"sku":"SKU-2","name":"Widget","quantity":4}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/items/SKU-1", strings.NewReader(body)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

type unavailableStore struct {
	*fakeStore
}

func (unavailableStore) PingContext(context.Context) error {
	return errors.New("database unavailable")
}

func TestReadinessReportsUnavailableDependency(t *testing.T) {
	handler := NewHandler(unavailableStore{newFakeStore()}).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
