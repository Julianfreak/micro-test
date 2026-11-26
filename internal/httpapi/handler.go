package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Julianfreak/micro-test/internal/inventory"
)

type inventoryUseCases interface {
	List(context.Context) ([]inventory.Item, error)
	Get(context.Context, string) (inventory.Item, error)
	Create(context.Context, inventory.Item) (inventory.Item, error)
	CreateIdempotently(context.Context, string, [32]byte, inventory.Item) (inventory.Item, error)
	Update(context.Context, string, inventory.Item) (inventory.Item, error)
	Delete(context.Context, string) error
}

type ReadinessChecker interface {
	PingContext(context.Context) error
}

type Handler struct {
	service   inventoryUseCases
	readiness ReadinessChecker
}

func NewHandler(service inventoryUseCases, readiness ReadinessChecker) *Handler {
	return &Handler{service: service, readiness: readiness}
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
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /health/live [get]
func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "UP", Agent: "local"})
}

// @Summary      Check service readiness
// @Tags         health
// @Produce      json
// @Success      200 {object} healthResponse
// @Failure      503 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /health [get]
// @Router       /health/ready [get]
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.readiness != nil {
		if err := h.readiness.PingContext(r.Context()); err != nil {
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
// @Success      200 {array} inventory.Item
// @Failure      500 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /items [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
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
// @Success      200 {object} inventory.Item
// @Failure      404 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /items/{sku} [get]
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Get(r.Context(), r.PathValue("sku"))
	if errors.Is(err, inventory.ErrNotFound) {
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
// @Param        item body inventory.Item true "Inventory item"
// @Param        Idempotency-Key header string false "Optional key for safely retrying this request"
// @Success      201 {object} inventory.Item
// @Failure      400 {object} errorResponse
// @Failure      409 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /items [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var item inventory.Item
	if err := decodeItem(w, r, &item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rawKey := r.Header.Get("Idempotency-Key")
	key := strings.TrimSpace(rawKey)
	if (rawKey != "" && key == "") || len(key) > 255 {
		writeError(w, http.StatusBadRequest, "Idempotency-Key must contain between 1 and 255 bytes")
		return
	}

	var created inventory.Item
	var err error
	if key == "" {
		created, err = h.service.Create(r.Context(), item)
	} else {
		fingerprint, hashErr := hashCreateRequest(item)
		if hashErr != nil {
			slog.ErrorContext(r.Context(), "hash idempotent create request", "error", hashErr)
			writeError(w, http.StatusInternalServerError, "could not process idempotency key")
			return
		}
		created, err = h.service.CreateIdempotently(r.Context(), key, fingerprint, item)
	}
	if errors.Is(err, inventory.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, inventory.ErrIdempotencyKeyReuse) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, inventory.ErrConflict) {
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

func hashCreateRequest(item inventory.Item) ([32]byte, error) {
	payload, err := json.Marshal(struct {
		SKU      string `json:"sku"`
		Name     string `json:"name"`
		Quantity int32  `json:"quantity"`
	}{
		SKU:      item.SKU,
		Name:     item.Name,
		Quantity: item.Quantity,
	})
	if err != nil {
		return [32]byte{}, fmt.Errorf("encode inventory request: %w", err)
	}
	return sha256.Sum256(payload), nil
}

// @Summary      Update an inventory item
// @Tags         inventory
// @Accept       json
// @Produce      json
// @Param        sku path string true "Stock keeping unit"
// @Param        item body inventory.Item true "Updated inventory item; SKU must match the path"
// @Success      200 {object} inventory.Item
// @Failure      400 {object} errorResponse
// @Failure      404 {object} errorResponse
// @Failure      500 {object} errorResponse
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /items/{sku} [put]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var item inventory.Item
	if err := decodeItem(w, r, &item); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.service.Update(r.Context(), r.PathValue("sku"), item)
	if errors.Is(err, inventory.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, inventory.ErrNotFound) {
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
// @Failure      401 {object} errorResponse
// @Security     BearerAuth
// @Router       /items/{sku} [delete]
func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	err := h.service.Delete(r.Context(), r.PathValue("sku"))
	if errors.Is(err, inventory.ErrNotFound) {
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

func decodeItem(w http.ResponseWriter, r *http.Request, item *inventory.Item) error {
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
