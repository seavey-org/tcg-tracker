package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/codyseavey/tcg-tracker/backend/internal/api"
	"github.com/codyseavey/tcg-tracker/backend/internal/database"
	"github.com/codyseavey/tcg-tracker/backend/internal/services"
)

func main() {
	// Database path
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tcg_tracker.db"
	}

	// Initialize database
	if err := database.Initialize(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize services
	scryfallService := services.NewScryfallService()

	// Initialize Pokemon hybrid service (local data, prices via JustTCG PriceWorker)
	dataDir := os.Getenv("POKEMON_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	pokemonService, err := services.NewPokemonHybridService(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize Pokemon service: %v", err)
	}
	log.Printf("Loaded %d Pokemon cards from %d sets", pokemonService.GetCardCount(), pokemonService.GetSetCount())

	// Initialize OCR parser with Pokemon names from the database
	// This enables comprehensive name matching for all cards in the database
	pokemonNames := pokemonService.GetAllPokemonNames()
	services.InitPokemonNamesFromData(pokemonNames)

	// Initialize JustTCG service for condition-based pricing
	justTCGAPIKey := os.Getenv("JUSTTCG_API_KEY")
	justTCGDailyLimit := 100 // Default free tier limit
	if limitStr := os.Getenv("JUSTTCG_DAILY_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			justTCGDailyLimit = limit
		}
	}
	justTCGService := services.NewJustTCGService(justTCGAPIKey, justTCGDailyLimit)

	// Initialize price service (JustTCG only, no fallbacks)
	priceService := services.NewPriceService(justTCGService, database.GetDB())

	// Initialize price worker with JustTCG batch support
	priceWorker := services.NewPriceWorker(priceService, pokemonService, justTCGService)

	// Initialize image storage service
	imageStorageService := services.NewImageStorageService()

	// Initialize snapshot service for daily value tracking
	snapshotService := services.NewSnapshotService()

	// Initialize TCGPlayer sync service for bulk prepopulating TCGPlayerIDs
	tcgPlayerSync := services.NewTCGPlayerSyncService(justTCGService)

	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start price worker in background with panic recovery
	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("PANIC in price worker: %v - restarting in 30 seconds", r)
					}
				}()
				priceWorker.Start(ctx)
			}()

			select {
			case <-ctx.Done():
				return // Graceful shutdown
			case <-time.After(30 * time.Second):
				log.Println("Price worker restarting after panic recovery...")
			}
		}
	}()

	// Start snapshot service in background
	go snapshotService.Start(ctx)

	// Optionally sync missing TCGPlayerIDs on startup (if enabled)
	if os.Getenv("SYNC_TCGPLAYER_IDS_ON_STARTUP") == "true" {
		go func() {
			// Wait a bit for the server to be ready
			time.Sleep(5 * time.Second)
			log.Println("Starting TCGPlayerID sync on startup...")
			result, err := tcgPlayerSync.SyncMissingTCGPlayerIDs(ctx)
			if err != nil {
				log.Printf("TCGPlayerID sync failed: %v", err)
			} else if result != nil {
				log.Printf("TCGPlayerID sync completed: %d cards updated", result.CardsUpdated)
			}
		}()
	}

	// Setup router
	router := api.SetupRouter(scryfallService, pokemonService, priceWorker, priceService, imageStorageService, snapshotService, tcgPlayerSync, justTCGService)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server for graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Cancel the context to stop the price worker
	cancel()

	// Give outstanding requests a deadline to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
