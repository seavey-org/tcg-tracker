// Package metrics provides Prometheus metrics for the TCG Tracker application.
// Scrape these at /metrics for Grafana dashboards and alerting.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tcg_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Price Worker Metrics
	PriceUpdatesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tcg_price_updates_total",
			Help: "Total number of card prices updated",
		},
	)

	PriceUpdatesToday = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_price_updates_today",
			Help: "Number of card prices updated today (resets at midnight)",
		},
	)

	PriceQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_price_queue_size",
			Help: "Number of cards waiting in the priority refresh queue",
		},
	)

	PriceBatchDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tcg_price_batch_duration_seconds",
			Help:    "Time taken to process a price update batch",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 30, 60},
		},
	)

	// JustTCG API Metrics
	JustTCGRequestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tcg_justtcg_requests_total",
			Help: "Total number of JustTCG API requests made",
		},
	)

	JustTCGQuotaRemaining = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_justtcg_quota_remaining",
			Help: "Remaining JustTCG API requests for today",
		},
	)

	JustTCGQuotaLimit = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_justtcg_quota_limit",
			Help: "Daily JustTCG API request limit",
		},
	)

	JustTCGMonthlyQuotaRemaining = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_justtcg_monthly_quota_remaining",
			Help: "Remaining JustTCG API requests for this month",
		},
	)

	JustTCGMonthlyQuotaLimit = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_justtcg_monthly_quota_limit",
			Help: "Monthly JustTCG API request limit",
		},
	)

	// Collection Metrics
	CollectionCardsTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_collection_cards_total",
			Help: "Total number of cards in collection",
		},
	)

	CollectionValueUSD = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_collection_value_usd",
			Help: "Total estimated value of collection in USD",
		},
	)

	CollectionCardsByGame = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tcg_collection_cards_by_game",
			Help: "Number of cards in collection by game",
		},
		[]string{"game"},
	)

	CollectionValueByGame = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tcg_collection_value_by_game_usd",
			Help: "Collection value in USD by game",
		},
		[]string{"game"},
	)

	// Card Database Metrics
	CardDatabaseSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tcg_card_database_size",
			Help: "Number of unique cards in the database",
		},
	)

	// OCR Metrics
	OCRRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_ocr_requests_total",
			Help: "Total number of OCR identification requests",
		},
		[]string{"type", "result"}, // type: "server" or "client", result: "success" or "failed"
	)

	OCRProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tcg_ocr_processing_duration_seconds",
			Help:    "Time taken to process OCR requests",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
	)

	// Translation Metrics
	TranslationRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_translation_requests_total",
			Help: "Total translation requests by source",
		},
		[]string{"source"}, // "static", "cache", "api"
	)

	TranslationAPILatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tcg_translation_api_latency_seconds",
			Help:    "Google Cloud Translation API call latency",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5},
		},
	)

	TranslationCacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tcg_translation_cache_hits_total",
			Help: "Translation cache hit count",
		},
	)

	TranslationCacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tcg_translation_cache_misses_total",
			Help: "Translation cache miss count",
		},
	)

	TranslationErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_translation_errors_total",
			Help: "Translation errors by type",
		},
		[]string{"type"}, // "auth", "api", "cache"
	)

	// Gemini Translation Metrics
	GeminiRequestsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tcg_gemini_requests_total",
			Help: "Total Gemini API translation requests",
		},
	)

	GeminiAPILatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tcg_gemini_api_latency_seconds",
			Help:    "Gemini API call latency",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5},
		},
	)

	GeminiErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_gemini_errors_total",
			Help: "Gemini API errors by type",
		},
		[]string{"type"}, // "network", "read", "api", "parse", "schema", "empty", "no_candidates"
	)

	GeminiConfidenceHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tcg_gemini_confidence",
			Help:    "Gemini response confidence scores for best candidate",
			Buckets: []float64{0.1, 0.3, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 1.0},
		},
	)

	// Translation Decision Metrics
	TranslationDecisions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tcg_translation_decisions_total",
			Help: "Translation source decisions",
		},
		[]string{"source"}, // "static", "cache", "gemini", "google_api", "failed", "skipped"
	)
)
