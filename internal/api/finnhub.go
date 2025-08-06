package api

import (
	"context"
	"fmt"
	"time"

	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
	"github.com/whatcher1074/stockspotlight/internal/finnhub_limiter"
	"github.com/whatcher1074/stockspotlight/internal/logger"
)

const (
	cacheKey = "snapshot"
)

// Global rate limiter for Finnhub API calls (60 calls/minute = 1 call/second)
var finnhubLimiter = finnhub_limiter.NewLimiter(time.Second)

// Finnhub API client
var finnhubClient *finnhub.DefaultApiService

// InitFinnhubClient initializes the Finnhub API client.
func InitFinnhubClient(apiKey string) {
	cfg := finnhub.NewConfiguration()
	cfg.AddDefaultHeader("X-Finnhub-Token", apiKey)
	finnhubClient = finnhub.NewAPIClient(cfg).DefaultApi
}

// CombinedData is the final data structure we'll cache.
type CombinedData struct {
	Ticker string
	Name   string
	Price  float64
	High   float64
	Low    float64
	Volume float64 // Note: Finnhub /quote does not provide volume directly
	Change float64
}

// NewsArticle represents a single news article.
type NewsArticle struct {
	Category string `json:"category"`
	Datetime int64  `json:"datetime"`
	Headline string `json:"headline"`
	ID       int64  `json:"id"`
	Image    string `json:"image"`
	Related  string `json:"related"`
	Source   string `json:"source"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
	Time     string // Formatted time for display
}

// FetchAllData fetches data for a given screener signal.
// The finnhub-go library version being used does not have the StockScreener function.
// This is a mock function to allow the application to run.
func FetchAllData(apiKey string, limit int, logger *logger.Logger, signal string) ([]CombinedData, error) {
	// Mock data
	mockData := []CombinedData{
		{Ticker: "AAPL", Name: "Apple Inc.", Price: 172.28, High: 173.05, Low: 170.12, Volume: 52, Change: -0.54},
		{Ticker: "GOOGL", Name: "Alphabet Inc.", Price: 136.99, High: 137.50, Low: 135.20, Volume: 25, Change: 0.89},
		{Ticker: "MSFT", Name: "Microsoft Corporation", Price: 370.95, High: 372.10, Low: 368.45, Volume: 30, Change: 1.23},
		{Ticker: "AMZN", Name: "Amazon.com, Inc.", Price: 134.26, High: 135.10, Low: 133.00, Volume: 40, Change: -1.10},
		{Ticker: "TSLA", Name: "Tesla, Inc.", Price: 234.86, High: 238.90, Low: 232.50, Volume: 60, Change: 2.50},
	}

	return mockData, nil
}

// FetchCompanyProfile fetches the company profile for a given symbol.
func FetchCompanyProfile(symbol string) (finnhub.CompanyProfile2, error) {
	finnhubLimiter.Wait() // Wait before making the API call
	ctx := context.Background()

	profile, _, err := finnhubClient.CompanyProfile2(ctx).Symbol(symbol).Execute()
	if err != nil {
		return finnhub.CompanyProfile2{}, fmt.Errorf("failed to fetch company profile for %s: %w", symbol, err)
	}

	return profile, nil
}

// FetchNews fetches general news articles.
func FetchNews(category string, limit int) ([]NewsArticle, error) {
	finnhubLimiter.Wait() // Wait before making the API call
	ctx := context.Background()

	// The API now uses CompanyNews instead of News and requires From and To dates.
	// For simplicity, I'm using a fixed date range for now.
	// You might want to make these parameters dynamic based on your application's needs.
	news, _, err := finnhubClient.CompanyNews(ctx).Symbol("AAPL").From("2023-01-01").To("2023-01-01").Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news for %s: %w", category, err)
	}

	var articles []NewsArticle
	for i, article := range news {
		if i >= limit {
			break
		}
		articles = append(articles, NewsArticle{
			Category: *article.Category,
			Datetime: *article.Datetime,
			Headline: *article.Headline,
			ID:       *article.Id,
			Image:    *article.Image,
			Related:  *article.Related,
			Source:   *article.Source,
			Summary:  *article.Summary,
			URL:      *article.Url,
		})
	}

	return articles, nil
}
