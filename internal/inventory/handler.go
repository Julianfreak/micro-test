package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Handler struct {
	store Store
}

func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.ready)
	mux.HandleFunc("GET /health/live", h.live)
	mux.HandleFunc("GET /health/ready", h.ready)
	mux.HandleFunc("GET /items", h.list)
	mux.HandleFunc("POST /items", h.create)
	mux.HandleFunc("GET /items/{sku}", h.get)
	mux.HandleFunc("PUT /items/{sku}", h.update)
	mux.HandleFunc("DELETE /items/{sku}", h.delete)
	return mux
}

type healthResponse struct {
	Status string `json:"status"`
	Agent  string `json:"agent"`
}

// @Summary      Check whether the process is alive
// @Tags         health
// @Produce      json
// @Success      200 {object} healthResponse
// @Router       /health/live [get]
func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "UP", Agent: "local"})
}

// @Summary      Check service readiness
// @Tags         health
// @Produce      json
// @Success      200 {object} healthResponse
// @Failure      503 {object} errorResponse
// @Router       /health [get]
// @Router       /health/ready [get]
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if checker, ok := h.store.(interface{ PingContext(context.Context) error }); ok {
		if err := checker.PingContext(r.Context()); err != nil {
			slog.ErrorContext(r.Context(), "readiness check failed", "error", err)
			writeError(w, http.StatusServiceUnavailable, "service is not ready")
			return
		}
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "UP", Agent: "local"})
}

// @Summary      List inventory items
// @Tags         inventory
// @Produce      json
// @Success      200 {array} Item
// @Failure      500 {object} errorResponse
// @Router       /items [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list inventory items", "error", err)
		writeError(w, http.StatusInternalServerError, "could not list inventory")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// @Summary      Get an inventory item
// @Tags         inventory
// @Produce      json
// @Param        sku path string true "Stock keeping unit"
// @Success      200 {object} Item
// @Failure      404 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /items/{sku} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.Get(r.Context(), r.PathValue("sku"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "get inventory item", "error", err)
		writeError(w, http.StatusInternalServerError, "could not get inventory item")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// @Summary      Create an inventory item
// @Tags         inventory
// @Accept       json
// @Produce      json
// @Param        item body Item true "Inventory item"
// @Success      201 {object} Item
// @Failure      400 {object} errorResponse
// @Failure      409 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /items [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var item Item
	if err := decodeItem(w, r, &item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateItem(item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.store.Create(r.Context(), item)
	if errors.Is(err, ErrConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "create inventory item", "error", err)
		writeError(w, http.StatusInternalServerError, "could not create inventory item")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// @Summary      Update an inventory item
// @Tags         inventory
// @Accept       json
// @Produce      json
// @Param        sku path string true "Stock keeping unit"
// @Param        item body Item true "Updated inventory item; SKU must match the path"
// @Success      200 {object} Item
// @Failure      400 {object} errorResponse
// @Failure      404 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /items/{sku} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var item Item
	if err := decodeItem(w, r, &item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if item.SKU != r.PathValue("sku") {
		writeError(w, http.StatusBadRequest, "sku in request body must match URL")
		return
	}
	if err := validateItem(item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.store.Update(r.Context(), r.PathValue("sku"), item)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "update inventory item", "error", err)
		writeError(w, http.StatusInternalServerError, "could not update inventory item")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// @Summary      Delete an inventory item
// @Tags         inventory
// @Param        sku path string true "Stock keeping unit"
// @Success      204
// @Failure      404 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Router       /items/{sku} [delete]
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	err := h.store.Delete(r.Context(), r.PathValue("sku"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "delete inventory item", "error", err)
		writeError(w, http.StatusInternalServerError, "could not delete inventory item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeItem(w http.ResponseWriter, r *http.Request, item *Item) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(item); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func validateItem(item Item) error {
	if strings.TrimSpace(item.SKU) == "" || len(item.SKU) > 64 {
		return errors.New("sku is required and must be at most 64 characters")
	}
	if strings.TrimSpace(item.Name) == "" || len(item.Name) > 200 {
		return errors.New("name is required and must be at most 200 characters")
	}
	if item.Quantity < 0 {
		return errors.New("quantity must not be negative")
	}
	return nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write HTTP response", "error", err)
	}
}
