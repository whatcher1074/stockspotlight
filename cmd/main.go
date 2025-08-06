// File: cmd/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/template"
	"time"

	"github.com/whatcher1074/stockspotlight/internal/api"
	"github.com/whatcher1074/stockspotlight/internal/cache"
	"github.com/whatcher1074/stockspotlight/internal/config"
	"github.com/whatcher1074/stockspotlight/internal/health"
	"github.com/whatcher1074/stockspotlight/internal/logger"
	// finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
)

var (
	indexTemplate          *template.Template
	stockTableTemplate     *template.Template
	gainersTableTemplate   *template.Template
	losersTableTemplate    *template.Template
	companyProfileTemplate *template.Template
	newsFeedTemplate       *template.Template
)

func main() {
	// Load config
	cfg, err := config.Load("internal/config/app.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.New("logs/app.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	appLogger.Info("App starting...")

	// Defer logger cleanup
	defer func() {
		appLogger.Info("Shutting down logger...")
		if err := appLogger.Close(); err != nil {
			log.Printf("Error closing logger: %v", err)
		}
	}()

	// Initialize Finnhub client
	api.InitFinnhubClient(cfg.APIKey)

	// Load HTML templates
	indexTemplate = template.Must(template.ParseFiles("static/index.html"))
	stockTableTemplate = template.Must(template.ParseFiles("static/stock_table.html"))
	gainersTableTemplate = template.Must(template.ParseFiles("static/gainers_table.html"))
	losersTableTemplate = template.Must(template.ParseFiles("static/losers_table.html"))
	companyProfileTemplate = template.Must(template.ParseFiles("static/company_profile.html"))
	newsFeedTemplate = template.Must(template.ParseFiles("static/news_feed.html"))

	// Setup cache
	c := cache.New()

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Static assets (Bootstrap, etc.)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health check
	mux.HandleFunc("/healthz", serveHealthz)

	// Log management endpoints
	mux.HandleFunc("/logs/status", func(w http.ResponseWriter, r *http.Request) {
		stats, err := appLogger.GetStats()
		if err != nil {
			appLogger.Errorf("Error getting log stats: %v", err)
			http.Error(w, fmt.Sprintf("Error getting log stats: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{
			"currentSize": "%s",
			"currentAge": "%v",
			"rotatedFiles": %d,
			"totalSize": "%s",
			"maxSize": "%s",
			"maxAge": "%v",
			"maxFiles": %d,
			"status": "healthy"
		}`,
			stats.FormatSize(stats.CurrentSize),
			stats.CurrentAge.Round(time.Minute),
			stats.RotatedCount,
			stats.FormatSize(stats.TotalSize),
			stats.FormatSize(logger.MaxLogFileSize),
			logger.MaxLogAge,
			logger.MaxLogFiles,
		)
		w.Write([]byte(response))
		appLogger.Infof("Log status requested: %s", stats.String())
	})

	mux.HandleFunc("/logs/rotate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appLogger.Info("Manual log rotation requested via API")
		err := appLogger.ForceRotate()
		if err != nil {
			appLogger.Errorf("Error rotating logs: %v", err)
			http.Error(w, fmt.Sprintf("Error rotating logs: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Log rotation completed"}`))
		appLogger.Info("Manual log rotation completed successfully")
	})

	mux.HandleFunc("/logs/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		appLogger.Info("Manual log cleanup requested via API")
		err := appLogger.CleanupOldLogs()
		if err != nil {
			appLogger.Errorf("Error cleaning up logs: %v", err)
			http.Error(w, fmt.Sprintf("Error cleaning up logs: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Log cleanup completed"}`))
		appLogger.Info("Manual log cleanup completed successfully")
	})

	// Generic handler function for screener data (most active, gainers, losers)
	createScreenerHandler := func(signal string, tmpl *template.Template, cacheKey string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			appLogger.Infof("Request received for %s", signal)
			now := time.Now().Format("15:04:05")

			cachedData, found := c.Get(cacheKey)
			var displayData []api.CombinedData

			if found {
				if d, ok := cachedData.([]api.CombinedData); ok {
					displayData = d
					appLogger.Infof("Using cached data for %s with %d items", signal, len(d))
				} else {
					appLogger.Errorf("Cached data type mismatch for %s", signal)
					found = false
				}
			}

			// If data not found in cache or expired, fetch it
			if !found {
				appLogger.Infof("Fetching fresh data for %s", signal)
				data, err := api.FetchAllData(cfg.APIKey, cfg.TickerLimit, appLogger, signal)
				if err != nil {
					appLogger.Errorf("Failed to fetch %s data: %v", signal, err)
					pageData := map[string]interface{}{
						"HasData":   false,
						"ErrorMsg":  fmt.Sprintf("Failed to load %s data: %v", signal, err),
						"Timestamp": now,
					}
					tmpl.Execute(w, pageData)
					return
				}
				c.Set(cacheKey, data, time.Duration(cfg.CacheTTL)*time.Second)
				displayData = data
				appLogger.Infof("Fetched %d items for %s, cached for %d seconds", len(data), signal, cfg.CacheTTL)
			}

			pageData := map[string]interface{}{
				"Data":      displayData,
				"HasData":   len(displayData) > 0,
				"ErrorMsg":  "",
				"Timestamp": now,
			}

			err := tmpl.Execute(w, pageData)
			if err != nil {
				appLogger.Errorf("Template render failed for %s: %v", signal, err)
			}
		}
	}

	// Stock screener endpoints
	mux.HandleFunc("/data/most-active", createScreenerHandler("most_active", stockTableTemplate, "most_active_snapshot"))
	mux.HandleFunc("/data/gainers", createScreenerHandler("gainers", gainersTableTemplate, "gainers_snapshot"))
	mux.HandleFunc("/data/losers", createScreenerHandler("losers", losersTableTemplate, "losers_snapshot"))

	// Company Profile endpoint
	mux.HandleFunc("/data/profile", func(w http.ResponseWriter, r *http.Request) {
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "AAPL" // Default to Apple
		}
		
		appLogger.Infof("Request received for company profile: %s", symbol)
		now := time.Now().Format("15:04:05")
		cacheKey := fmt.Sprintf("profile_%s", symbol)

		cachedData, found := c.Get(cacheKey)
		var profileData map[string]interface{}

		if found {
			if d, ok := cachedData.(map[string]interface{}); ok {
				profileData = d
				appLogger.Infof("Using cached profile data for %s", symbol)
			} else {
				found = false
			}
		}
		
		if !found {
			appLogger.Infof("Fetching fresh profile data for %s", symbol)
			
			// Mock profile data based on symbol
			profiles := map[string]map[string]interface{}{
				"AAPL": {
					"Name":     "Apple Inc.",
					"Industry": "Technology Hardware, Storage & Peripherals",
					"WebURL":   "https://www.apple.com",
					"Logo":     "https://logo.clearbit.com/apple.com",
				},
				"MSFT": {
					"Name":     "Microsoft Corporation",
					"Industry": "Systems Software",
					"WebURL":   "https://www.microsoft.com",
					"Logo":     "https://logo.clearbit.com/microsoft.com",
				},
				"GOOGL": {
					"Name":     "Alphabet Inc.",
					"Industry": "Interactive Media & Services",
					"WebURL":   "https://www.google.com",
					"Logo":     "https://logo.clearbit.com/google.com",
				},
				"TSLA": {
					"Name":     "Tesla, Inc.",
					"Industry": "Automobiles",
					"WebURL":   "https://www.tesla.com",
					"Logo":     "https://logo.clearbit.com/tesla.com",
				},
			}
			
			if profile, exists := profiles[symbol]; exists {
				profileData = profile
			} else {
				profileData = map[string]interface{}{
					"Name":     fmt.Sprintf("%s Corporation", symbol),
					"Industry": "Technology",
					"WebURL":   fmt.Sprintf("https://www.%s.com", symbol),
					"Logo":     fmt.Sprintf("https://logo.clearbit.com/%s.com", symbol),
				}
			}
			
			c.Set(cacheKey, profileData, time.Duration(cfg.CacheTTL)*time.Second)
			appLogger.Infof("Cached profile data for %s", symbol)
		}

		// Determine exchange based on symbol (mock logic)
		exchange := "NASDAQ"
		if len(symbol) > 0 && (symbol[0] >= 'A' && symbol[0] <= 'M') {
			exchange = "NYSE"
		}

		pageData := map[string]interface{}{
			"Data":      profileData,
			"HasData":   profileData != nil,
			"ErrorMsg":  "",
			"Timestamp": now,
			"Name":      profileData["Name"],
			"Ticker":    symbol,
			"Exchange":  exchange,
			"Industry":  profileData["Industry"],
			"WebURL":    profileData["WebURL"],
			"Logo":      profileData["Logo"],
		}

		err := companyProfileTemplate.Execute(w, pageData)
		if err != nil {
			appLogger.Errorf("Template render failed for profile: %v", err)
		}
	})

	// News Feed endpoint
	mux.HandleFunc("/data/news", func(w http.ResponseWriter, r *http.Request) {
		category := r.URL.Query().Get("category")
		if category == "" {
			category = "general"
		}
		
		appLogger.Infof("Request received for news: %s", category)
		now := time.Now().Format("15:04:05")
		cacheKey := fmt.Sprintf("news_%s", category)

		cachedData, found := c.Get(cacheKey)
		var displayData []map[string]interface{}

		if found {
			if d, ok := cachedData.([]map[string]interface{}); ok {
				displayData = d
				appLogger.Infof("Using cached news data for %s with %d articles", category, len(d))
			} else {
				found = false
			}
		}

		if !found {
			appLogger.Infof("Fetching fresh news data for %s", category)
			
			// Mock news data based on category
			newsData := map[string][]map[string]interface{}{
				"general": {
					{
						"Headline": "Stock Market Reaches New Highs Amid Economic Optimism",
						"URL":      "https://example.com/news1",
						"Source":   "Financial Times",
						"Time":     time.Now().Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "Federal Reserve Maintains Interest Rates",
						"URL":      "https://example.com/news3",
						"Source":   "Bloomberg",
						"Time":     time.Now().Add(-2*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "Global Markets Show Strong Recovery Signs",
						"URL":      "https://example.com/news4",
						"Source":   "Reuters",
						"Time":     time.Now().Add(-3*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
				},
				"tech": {
					{
						"Headline": "Tech Giants Report Strong Quarterly Earnings",
						"URL":      "https://example.com/tech1", 
						"Source":   "TechCrunch",
						"Time":     time.Now().Add(-1*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "AI Innovation Drives Tech Sector Growth",
						"URL":      "https://example.com/tech2",
						"Source":   "Wired",
						"Time":     time.Now().Add(-2*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "Cloud Computing Revenue Surges 40%",
						"URL":      "https://example.com/tech3",
						"Source":   "Ars Technica",
						"Time":     time.Now().Add(-4*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
				},
				"finance": {
					{
						"Headline": "Banking Sector Shows Resilience in Q4",
						"URL":      "https://example.com/fin1",
						"Source":   "Wall Street Journal",
						"Time":     time.Now().Add(-30*time.Minute).Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "Cryptocurrency Market Volatility Continues",
						"URL":      "https://example.com/fin2",
						"Source":   "CoinDesk",
						"Time":     time.Now().Add(-1*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
					{
						"Headline": "Corporate Bond Yields Rise Amid Inflation Concerns",
						"URL":      "https://example.com/fin3",
						"Source":   "Financial Times",
						"Time":     time.Now().Add(-3*time.Hour).Format("Jan 2, 2006 15:04 MST"),
					},
				},
			}
			
			if articles, exists := newsData[category]; exists {
				displayData = articles
			} else {
				displayData = newsData["general"] // Fallback to general
			}
			
			c.Set(cacheKey, displayData, time.Duration(cfg.CacheTTL)*time.Second)
			appLogger.Infof("Cached %d news articles for %s", len(displayData), category)
		}

		pageData := map[string]interface{}{
			"Data":      displayData,
			"HasData":   len(displayData) > 0,
			"ErrorMsg":  "",
			"Timestamp": now,
		}

		err := newsFeedTemplate.Execute(w, pageData)
		if err != nil {
			appLogger.Errorf("Template render failed for news: %v", err)
		}
	})

	// UI entry point
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		appLogger.Info("Serving UI /")
		err := indexTemplate.Execute(w, nil)
		if err != nil {
			appLogger.Errorf("Index template render failed: %v", err)
		}
	})

	// Configure server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		appLogger.Infof("Server running at http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v\n", port, err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	appLogger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatalf("Server shutdown failed: %v", err)
	}

	appLogger.Info("Server gracefully stopped")
}

func serveHealthz(w http.ResponseWriter, r *http.Request) {
	health.Handler(w, r)
}
