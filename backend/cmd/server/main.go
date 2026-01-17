package main

import (
	"context"
	"log"
	"os"
	"strconv"

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

	// Initialize Pokemon hybrid service (local data + TCGdex for prices)
	dataDir := os.Getenv("POKEMON_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	pokemonService, err := services.NewPokemonHybridService(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize Pokemon service: %v", err)
	}
	log.Printf("Loaded %d Pokemon cards from %d sets", pokemonService.GetCardCount(), pokemonService.GetSetCount())

	// Initialize price worker
	dailyLimit := 100 // Default daily API request limit
	if limitStr := os.Getenv("POKEMON_PRICE_DAILY_LIMIT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			dailyLimit = limit
		}
	}
	priceWorker := services.NewPriceWorker(pokemonService, dailyLimit)

	// Start price worker in background
	ctx := context.Background()
	go priceWorker.Start(ctx)

	// Setup router
	router := api.SetupRouter(scryfallService, pokemonService, priceWorker)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
