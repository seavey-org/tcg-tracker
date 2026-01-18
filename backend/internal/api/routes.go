package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/codyseavey/tcg-tracker/backend/internal/api/handlers"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

func SetupRouter(scryfallService *services.ScryfallService, pokemonService *services.PokemonHybridService, priceWorker *services.PriceWorker, priceService *services.PriceService, imageStorageService *services.ImageStorageService) *gin.Engine {
	router := gin.Default()

	// Get frontend dist path from env
	frontendPath := os.Getenv("FRONTEND_DIST_PATH")
	serveFrontend := frontendPath != "" && dirExists(frontendPath)

	// CORS configuration - allow origins from environment or use defaults
	config := cors.DefaultConfig()
	if corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); corsOrigins != "" {
		config.AllowOrigins = strings.Split(corsOrigins, ",")
	} else {
		config.AllowOrigins = []string{"http://localhost:5173", "http://localhost:3000"}
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.AllowCredentials = false // Explicitly set
	router.Use(cors.New(config))

	// Initialize handlers
	cardHandler := handlers.NewCardHandler(scryfallService, pokemonService)
	collectionHandler := handlers.NewCollectionHandler(scryfallService, pokemonService, imageStorageService)
	priceHandler := handlers.NewPriceHandler(priceWorker, priceService)

	// Serve scanned images
	if imageStorageService != nil {
		router.Static("/images/scanned", imageStorageService.GetStorageDir())
	}

	// API routes
	api := router.Group("/api")
	{
		// Card routes
		cards := api.Group("/cards")
		{
			cards.GET("/search", cardHandler.SearchCards)
			cards.GET("/:id", cardHandler.GetCard)
			cards.GET("/:id/prices", priceHandler.GetCardPrices)
			cards.POST("/identify", cardHandler.IdentifyCard)
			cards.POST("/identify-image", cardHandler.IdentifyCardFromImage)
			cards.POST("/identify-set", cardHandler.IdentifySetFromImage)
			cards.GET("/ocr-status", cardHandler.GetOCRStatus)
			cards.POST("/:id/refresh-price", priceHandler.RefreshCardPrice)
		}

		// Collection routes
		collection := api.Group("/collection")
		{
			collection.GET("", collectionHandler.GetCollection)
			collection.POST("", collectionHandler.AddToCollection)
			collection.PUT("/:id", collectionHandler.UpdateCollectionItem)
			collection.DELETE("/:id", collectionHandler.DeleteCollectionItem)
			collection.GET("/stats", collectionHandler.GetStats)
			collection.POST("/refresh-prices", collectionHandler.RefreshPrices)
		}

		// Price routes
		prices := api.Group("/prices")
		{
			prices.GET("/status", priceHandler.GetPriceStatus)
		}
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Serve frontend static files
	if serveFrontend {
		indexPath := filepath.Join(frontendPath, "index.html")

		// Serve static assets
		router.Static("/assets", filepath.Join(frontendPath, "assets"))

		// Serve other static files (favicon, etc.)
		router.StaticFile("/vite.svg", filepath.Join(frontendPath, "vite.svg"))

		// Serve root index.html
		router.GET("/", func(c *gin.Context) {
			c.File(indexPath)
		})

		// SPA fallback - serve index.html for all non-API routes
		router.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path

			// Don't serve index.html for API routes
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}

			// Serve index.html for SPA routing
			c.File(indexPath)
		})
	}

	return router
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
