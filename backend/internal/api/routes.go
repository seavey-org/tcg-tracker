package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/codyseavey/tcg-tracker/backend/internal/api/handlers"
	"github.com/codyseavey/tcg-tracker/backend/internal/metrics"
	"github.com/codyseavey/tcg-tracker/backend/internal/middleware"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

func SetupRouter(scryfallService *services.ScryfallService, pokemonService *services.PokemonHybridService, geminiService *services.GeminiService, priceWorker *services.PriceWorker, priceService *services.PriceService, imageStorageService *services.ImageStorageService, snapshotService *services.SnapshotService, tcgPlayerSync *services.TCGPlayerSyncService, justTCG *services.JustTCGService) *gin.Engine {
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

	// Prometheus metrics middleware (must be before routes)
	router.Use(metrics.HTTPMetrics())

	// Initialize handlers
	cardHandler := handlers.NewCardHandler(scryfallService, pokemonService, geminiService)
	collectionHandler := handlers.NewCollectionHandler(scryfallService, pokemonService, imageStorageService, snapshotService, priceWorker)
	priceHandler := handlers.NewPriceHandler(priceWorker, priceService)
	adminHandler := handlers.NewAdminHandler(tcgPlayerSync, justTCG)

	// Serve scanned images
	if imageStorageService != nil {
		router.Static("/images/scanned", imageStorageService.GetStorageDir())
	}

	// Admin key auth middleware for protected routes
	adminAuth := middleware.AdminKeyAuth()

	// API routes
	api := router.Group("/api")
	{
		// Auth routes (public)
		auth := api.Group("/auth")
		{
			auth.GET("/status", middleware.GetAuthStatus)
			auth.POST("/verify", middleware.VerifyAdminKey)
		}

		// Card routes (all public)
		cards := api.Group("/cards")
		{
			cards.GET("/search", cardHandler.SearchCards)
			cards.GET("/:id", cardHandler.GetCard)
			cards.GET("/:id/prices", priceHandler.GetCardPrices)
			cards.POST("/identify-image", cardHandler.IdentifyCardFromImage)
			cards.POST("/:id/refresh-price", priceHandler.RefreshCardPrice)
		}

		// Collection routes
		collection := api.Group("/collection")
		{
			// Public routes (read-only)
			collection.GET("", collectionHandler.GetCollection)
			collection.GET("/grouped", collectionHandler.GetGroupedCollection)
			collection.GET("/stats", collectionHandler.GetStats)
			collection.GET("/stats/history", collectionHandler.GetValueHistory)

			// Protected routes (require admin key)
			collection.POST("", adminAuth, collectionHandler.AddToCollection)
			collection.PUT("/:id", adminAuth, collectionHandler.UpdateCollectionItem)
			collection.DELETE("/:id", adminAuth, collectionHandler.DeleteCollectionItem)
			collection.POST("/refresh-prices", adminAuth, collectionHandler.RefreshPrices)
		}

		// Price routes (public)
		prices := api.Group("/prices")
		{
			prices.GET("/status", priceHandler.GetPriceStatus)
		}

		// Admin routes (protected)
		admin := api.Group("/admin")
		admin.Use(adminAuth)
		{
			// TCGPlayerID sync endpoints
			admin.POST("/sync-tcgplayer-ids", adminHandler.SyncTCGPlayerIDs)
			admin.POST("/sync-tcgplayer-ids/blocking", adminHandler.SyncTCGPlayerIDsBlocking)
			admin.POST("/sync-tcgplayer-ids/set/:setName", adminHandler.SyncSetTCGPlayerIDs)
			admin.GET("/sync-tcgplayer-ids/status", adminHandler.GetSyncStatus)
		}
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

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
