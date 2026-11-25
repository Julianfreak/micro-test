package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Julianfreak/micro-test/internal/inventory"
)

type fakeUseCases struct {
	items     map[string]inventory.Item
	createErr error
	listErr   error
	getErr    error
	updateErr error
	deleteErr error
}

func newFakeUseCases() *fakeUseCases {
	return &fakeUseCases{items: make(map[string]inventory.Item)}
}

func (s *fakeUseCases) List(context.Context) ([]inventory.Item, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	items := make([]inventory.Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}

func (s *fakeUseCases) Get(_ context.Context, sku string) (inventory.Item, error) {
	if s.getErr != nil {
		return inventory.Item{}, s.getErr
	}
	item, ok := s.items[sku]
	if !ok {
		return inventory.Item{}, inventory.ErrNotFound
	}
	return item, nil
}

func (s *fakeUseCases) Create(_ context.Context, item inventory.Item) (inventory.Item, error) {
	if s.createErr != nil {
		return inventory.Item{}, s.createErr
	}
	if _, ok := s.items[item.SKU]; ok {
		return inventory.Item{}, inventory.ErrConflict
	}
	s.items[item.SKU] = item
	return item, nil
}

func (s *fakeUseCases) Update(_ context.Context, sku string, item inventory.Item) (inventory.Item, error) {
	if s.updateErr != nil {
		return inventory.Item{}, s.updateErr
	}
	if _, ok := s.items[sku]; !ok {
		return inventory.Item{}, inventory.ErrNotFound
	}
	s.items[sku] = item
	return item, nil
}

func (s *fakeUseCases) Delete(_ context.Context, sku string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.items[sku]; !ok {
		return inventory.ErrNotFound
	}
	delete(s.items, sku)
	return nil
}

type fakeReadiness struct {
	err error
}

func (r fakeReadiness) PingContext(context.Context) error {
	return r.err
}

func TestHandlerCreateAndGetItem(t *testing.T) {
	service := newFakeUseCases()
	handler := NewHandler(service, fakeReadiness{}).Routes()
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
	var item inventory.Item
	if err := json.Unmarshal(get.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item response: %v", err)
	}
	if item.SKU != "SKU-1" || item.Quantity != 4 {
		t.Fatalf("unexpected item response: %+v", item)
	}
}

func TestHandlerListItems(t *testing.T) {
	tests := []struct {
		name    string
		listErr error
		want    int
	}{
		{name: "success", want: http.StatusOK},
		{name: "application failure", listErr: errors.New("storage unavailable"), want: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newFakeUseCases()
			service.listErr = test.listErr
			handler := NewHandler(service, fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items", nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestHandlerGetMapsMissingAndUnexpectedErrors(t *testing.T) {
	tests := []struct {
		name   string
		getErr error
		seed   bool
		want   int
	}{
		{name: "not found", want: http.StatusNotFound},
		{name: "application failure", getErr: errors.New("storage unavailable"), want: http.StatusInternalServerError},
		{name: "success", seed: true, want: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newFakeUseCases()
			service.getErr = test.getErr
			if test.seed {
				service.items["SKU-1"] = inventory.Item{SKU: "SKU-1", Name: "Widget", Quantity: 2}
			}
			handler := NewHandler(service, fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items/SKU-1", nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestHandlerMapsApplicationErrorsToHTTPStatuses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid input", err: inventory.ErrInvalid, want: http.StatusBadRequest},
		{name: "duplicate SKU", err: inventory.ErrConflict, want: http.StatusConflict},
		{name: "unexpected application failure", err: errors.New("storage unavailable"), want: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newFakeUseCases()
			service.createErr = test.err
			handler := NewHandler(service, fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/items",
				strings.NewReader(`{"sku":"SKU-1","name":"Widget","quantity":1}`))

			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestHandlerUpdateMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name      string
		updateErr error
		seed      bool
		want      int
	}{
		{name: "invalid input", updateErr: inventory.ErrInvalid, want: http.StatusBadRequest},
		{name: "not found", want: http.StatusNotFound},
		{name: "application failure", updateErr: errors.New("storage unavailable"), seed: true, want: http.StatusInternalServerError},
		{name: "success", seed: true, want: http.StatusOK},
	}
	body := `{"sku":"SKU-1","name":"Updated widget","quantity":5}`

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newFakeUseCases()
			service.updateErr = test.updateErr
			if test.seed {
				service.items["SKU-1"] = inventory.Item{SKU: "SKU-1", Name: "Widget", Quantity: 2}
			}
			handler := NewHandler(service, fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, "/items/SKU-1", strings.NewReader(body))
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestHandlerDeleteMapsApplicationErrors(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		seed      bool
		want      int
	}{
		{name: "not found", want: http.StatusNotFound},
		{name: "application failure", deleteErr: errors.New("storage unavailable"), seed: true, want: http.StatusInternalServerError},
		{name: "success", seed: true, want: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newFakeUseCases()
			service.deleteErr = test.deleteErr
			if test.seed {
				service.items["SKU-1"] = inventory.Item{SKU: "SKU-1", Name: "Widget", Quantity: 2}
			}
			handler := NewHandler(service, fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/items/SKU-1", nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.want, response.Body)
			}
		})
	}
}

func TestHandlerRejectsMalformedJSON(t *testing.T) {
	handler := NewHandler(newFakeUseCases(), fakeReadiness{}).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(`{"sku":`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHandlerRejectsUnknownAndOversizedJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"sku":"SKU-1","name":"Widget","quantity":1,"extra":true}`},
		{name: "multiple values", body: `{"sku":"SKU-1","name":"Widget","quantity":1} {}`},
		{name: "oversized body", body: `{"name":"` + strings.Repeat("x", 1<<20) + `"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(newFakeUseCases(), fakeReadiness{}).Routes()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(test.body)))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandlerLiveness(t *testing.T) {
	handler := NewHandler(newFakeUseCases(), fakeReadiness{}).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestHandlerReadinessReportsDependencyFailure(t *testing.T) {
	handler := NewHandler(newFakeUseCases(), fakeReadiness{err: errors.New("database unavailable")}).Routes()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}
