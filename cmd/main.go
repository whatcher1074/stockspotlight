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

	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
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

	// Helper function to fetch and render screener data
	/*
		fetchAndRenderScreener := func(w http.ResponseWriter, r *http.Request, signal string, tmpl *template.Template, cacheKey string) {
			appLogger.Infof("Serving data for HTMX panel: %s", signal)
			cachedData, found := c.Get(cacheKey)

			var displayData []api.CombinedData
			if found {
				if d, ok := cachedData.([]api.CombinedData); ok {
					displayData = d
				} else {
					appLogger.Errorf("Cached data for %s is not of expected type []api.CombinedData", signal)
					found = false
				}
			}

			// If data not found in cache or expired, fetch it
			if !found {
				data, err := api.FetchAllData(cfg.APIKey, cfg.TickerLimit, appLogger, signal)
				if err != nil {
					appLogger.Errorf("Failed to fetch %s data: %v", signal, err)
					// Still render the template, but with an error message
					pageData := map[string]interface{}{
						"HasData":  false,
						"ErrorMsg": fmt.Sprintf("Failed to load %s data: %v", signal, err),
					}
					tmpl.Execute(w, pageData)
					return
				}
				c.Set(cacheKey, data, time.Duration(cfg.CacheTTL)*time.Second)
				displayData = data
				found = true
			}

			pageData := map[string]interface{}{
				"Data":     displayData,
				"HasData":  found,
				"Error":    !found,
				"ErrorMsg": "Stock data is not yet available. The application may be starting up, or there could be an issue with the data provider.",
			}
			err = tmpl.Execute(w, pageData)
			if err != nil {
				appLogger.Errorf("Template render failed for %s: %v", signal, err)
			}
		}
	*/

	// Helper function to fetch and render company profile data
	fetchAndRenderProfile := func(w http.ResponseWriter, r *http.Request, symbol string, tmpl *template.Template, cacheKey string) {
		appLogger.Info("Serving company profile for " + symbol)
		cachedData, found := c.Get(cacheKey)

		var displayData finnhub.CompanyProfile2
		if found {
			if d, ok := cachedData.(finnhub.CompanyProfile2); ok {
				displayData = d
			} else {
				appLogger.Errorf("Cached data for %s is not of expected type finnhub.CompanyProfile2", symbol)
				found = false
			}
		}

		if !found {
			data, err := api.FetchCompanyProfile(symbol)
			if err != nil {
				appLogger.Errorf("Failed to fetch company profile for %s: %v", symbol, err)
				pageData := map[string]interface{}{
					"HasData":  false,
					"ErrorMsg": fmt.Sprintf("Failed to load profile for %s: %v", symbol, err),
				}
				tmpl.Execute(w, pageData)
				return
			}
			c.Set(cacheKey, data, time.Duration(cfg.CacheTTL)*time.Second)
			displayData = data
			found = true
		}

		pageData := map[string]interface{}{
			"Data":     displayData,
			"HasData":  found,
			"Error":    !found,
			"ErrorMsg": "Company profile not yet available.",
			"Name":     displayData.Name,
			"Ticker":   displayData.Ticker,
			"Exchange": displayData.Exchange,
			"Industry": displayData.FinnhubIndustry,
			"WebURL":   displayData.Weburl,
			"Logo":     displayData.Logo,
		}
		err = tmpl.Execute(w, pageData)
		if err != nil {
			appLogger.Errorf("Template render failed for company profile: %v", err)
		}
	}

	// Helper function to fetch and render news data
	fetchAndRenderNews := func(w http.ResponseWriter, r *http.Request, category string, tmpl *template.Template, cacheKey string) {
		appLogger.Info("Serving news for category: " + category)
		cachedData, found := c.Get(cacheKey)

		var displayData []api.NewsArticle
		if found {
			if d, ok := cachedData.([]api.NewsArticle); ok {
				displayData = d
			} else {
				appLogger.Errorf("Cached data for %s is not of expected type []api.NewsArticle", category)
				found = false
			}
		}

		if !found {
			data, err := api.FetchNews(category, 5) // Limit to 5 news articles
			if err != nil {
				appLogger.Errorf("Failed to fetch news for %s: %v", category, err)
				pageData := map[string]interface{}{
					"HasData":  false,
					"ErrorMsg": fmt.Sprintf("Failed to load news for %s: %v", category, err),
				}
				tmpl.Execute(w, pageData)
				return
			}
			// Format datetime for display
			for i := range data {
				data[i].Time = time.Unix(data[i].Datetime, 0).Format("Jan 2, 2006 15:04 MST")
			}
			c.Set(cacheKey, data, time.Duration(cfg.CacheTTL)*time.Second)
			displayData = data
			found = true
		}

		pageData := map[string]interface{}{
			"Data":     displayData,
			"HasData":  found,
			"Error":    !found,
			"ErrorMsg": "News not yet available.",
		}
		err = tmpl.Execute(w, pageData)
		if err != nil {
			appLogger.Errorf("Template render failed for news: %v", err)
		}
	}

	/*
		mux.HandleFunc("/data/most-active", func(w http.ResponseWriter, r *http.Request) {
			fetchAndRenderScreener(w, r, "most_active", stockTableTemplate, "most_active_snapshot")
		})

		// Data endpoint for HTMX (Top Gainers)
		mux.HandleFunc("/data/gainers", func(w http.ResponseWriter, r *http.Request) {
			fetchAndRenderScreener(w, r, "gainers", gainersTableTemplate, "gainers_snapshot")
		})

		// Data endpoint for HTMX (Top Losers)
		mux.HandleFunc("/data/losers", func(w http.ResponseWriter, r *http.Request) {
			fetchAndRenderScreener(w, r, "losers", losersTableTemplate, "losers_snapshot")
		})
	*/

	// Data endpoint for HTMX (Company Profile)
	mux.HandleFunc("/data/profile", func(w http.ResponseWriter, r *http.Request) {
		fetchAndRenderProfile(w, r, "AAPL", companyProfileTemplate, "aapl_profile_snapshot")
	})

	// Data endpoint for HTMX (News Feed)
	mux.HandleFunc("/data/news", func(w http.ResponseWriter, r *http.Request) {
		fetchAndRenderNews(w, r, "general", newsFeedTemplate, "general_news_snapshot")
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
