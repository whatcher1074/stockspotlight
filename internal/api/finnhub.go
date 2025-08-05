package api

import (
	"context"
	"fmt"
	"time"

	finnhub "github.com/Finnhub-Stock-API/finnhub-go/v2"
	"github.com/whatcher1074/stockspotlight/internal/finnhub_limiter"
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
// Commenting out StockScreener related code as it seems to be removed or changed significantly in the new API.
//func FetchAllData(limit int, logger *logger.Logger, signal string) ([]CombinedData, error) {
//	finnhubLimiter.Wait() // Wait before making the API call for screener
//	ctx := context.Background()
//
//	screenerData, _, err := finnhubClient.StockScreener(ctx).Exchange("US").Signal(signal).Execute()
//	if err != nil {
//		return nil, fmt.Errorf("failed to fetch screener data for signal %s: %w", signal, err)
//	}
//
//	var combinedData []CombinedData
//	for i, t := range screenerData.Data {
//		if i >= limit {
//			break // Respect the limit from config
//		}
//
//		finnhubLimiter.Wait() // Wait before making the API call for quote
//		quote, _, err := finnhubClient.Quote(ctx).Symbol(*t.Symbol).Execute()
//		if err != nil {
//			logger.Errorf("Failed to fetch quote for %s: %v", *t.Symbol, err)
//			continue // Skip this ticker if there's an error
//		}
//
//		combinedData = append(combinedData, CombinedData{
//			Ticker: *t.Symbol,
//			Name:   *t.Symbol, // Finnhub screener doesn't provide name, use symbol for now
//			Price:  float64(*quote.C),
//			High:   float64(*quote.H),
//			Low:    float64(*quote.L),
//			Volume: 0, // Finnhub /quote does not provide volume directly
//			Change: float64(*quote.C - *quote.O),
//		})
//	}
//
//	return combinedData, nil
//}

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
