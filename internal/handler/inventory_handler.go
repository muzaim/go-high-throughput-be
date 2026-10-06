package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"indico-test-be/internal/broker"
	"indico-test-be/internal/model"
	"indico-test-be/internal/service"

	"github.com/gin-gonic/gin"
)

type InventoryHandler struct {
	inventoryService service.InventoryService
	broker           broker.SSEBroker
}

func NewInventoryHandler(inventoryService service.InventoryService, sseBroker broker.SSEBroker) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
		broker:           sseBroker,
	}
}

func (h *InventoryHandler) Reserve(c *gin.Context) {
	var req model.ReserveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error(), nil)
		return
	}

	resp, err := h.inventoryService.ReserveStock(c.Request.Context(), &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InventoryHandler) Confirm(c *gin.Context) {
	var req model.ConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "INVALID_INPUT", err.Error(), nil)
		return
	}

	resp, err := h.inventoryService.ConfirmReservation(c.Request.Context(), &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InventoryHandler) GetStock(c *gin.Context) {
	itemID := c.Query("item_id")
	if itemID == "" {
		h.respondError(c, http.StatusBadRequest, "MISSING_ITEM_ID", "item_id parameter is required", nil)
		return
	}

	resp, err := h.inventoryService.GetStock(c.Request.Context(), itemID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InventoryHandler) GetAllItems(c *gin.Context) {
	items, err := h.inventoryService.GetAllItems(c.Request.Context())
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.ItemListResponse{
		Status: "success",
		Items:  items,
	})
}

func (h *InventoryHandler) GetItemDetail(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		h.respondError(c, http.StatusBadRequest, "MISSING_ITEM_ID", "item id parameter is required", nil)
		return
	}

	item, err := h.inventoryService.GetItemDetail(c.Request.Context(), id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.ItemDetailResponse{
		Status: "success",
		Item:   *item,
	})
}

func (h *InventoryHandler) Stream(c *gin.Context) {
	itemID := c.Query("item_id")
	if itemID == "" {
		h.respondError(c, http.StatusBadRequest, "MISSING_ITEM_ID", "item_id parameter is required", nil)
		return
	}

	if h.broker == nil {
		h.respondError(c, http.StatusInternalServerError, "BROKER_UNAVAILABLE", "event broker unavailable", nil)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	ch, unsubscribe := h.broker.Subscribe(itemID)
	defer unsubscribe()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case <-ticker.C:
			c.SSEvent("", ": heartbeat\n")
			return true
		case publishedItemID, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("inventory_updated", gin.H{"item_id": publishedItemID})
			return true
		}
	})
}

func (h *InventoryHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrItemNotFound):
		h.respondError(c, http.StatusNotFound, "ITEM_NOT_FOUND", "item does not exist", nil)
	case errors.Is(err, model.ErrReservationNotFound):
		h.respondError(c, http.StatusNotFound, "RESERVATION_NOT_FOUND", "reservation does not exist", nil)
	case errors.Is(err, model.ErrInsufficientStock):
		h.respondError(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient inventory", nil)
	case errors.Is(err, model.ErrReservationAlreadyConfirmed):
		h.respondError(c, http.StatusConflict, "ALREADY_CONFIRMED", "reservation already confirmed", nil)
	case errors.Is(err, model.ErrReservationExpired):
		h.respondError(c, http.StatusConflict, "RESERVATION_EXPIRED", "reservation already expired", nil)
	case errors.Is(err, model.ErrInvalidQuantity):
		h.respondError(c, http.StatusBadRequest, "INVALID_QUANTITY", "quantity must be greater than 0", nil)
	default:
		h.respondError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected error occurred", nil)
	}
}

func (h *InventoryHandler) respondError(c *gin.Context, statusCode int, code string, message string, details any) {
	c.JSON(statusCode, model.NewErrorResponse(code, message, details))
}
