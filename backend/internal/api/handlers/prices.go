package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

type PriceHandler struct {
	priceWorker *services.PriceWorker
}

func NewPriceHandler(priceWorker *services.PriceWorker) *PriceHandler {
	return &PriceHandler{
		priceWorker: priceWorker,
	}
}

// GetPriceStatus returns the current API quota status
func (h *PriceHandler) GetPriceStatus(c *gin.Context) {
	status := h.priceWorker.GetStatus()
	c.JSON(http.StatusOK, status)
}

// RefreshCardPrice manually refreshes a single card's price
func (h *PriceHandler) RefreshCardPrice(c *gin.Context) {
	cardID := c.Param("id")

	if cardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "card id is required"})
		return
	}

	card, err := h.priceWorker.UpdateCard(cardID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if card == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "card not found or not a Pokemon card"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"card": card,
	})
}
