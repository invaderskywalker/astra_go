// astra/controllers/scrape.go
package controllers

import (
	"astra/astra/sources/storage"
	"astra/astra/utils/scraper"
	"astra/astra/utils/types"
	"context"
)

// ScrapeController manages web scraping requests
type ScrapeController struct {
	minio   *storage.MinIOClient
	scraper *scraper.Scraper
}

// NewScrapeController creates a new ScrapeController
func NewScrapeController(minio *storage.MinIOClient) (*ScrapeController, error) {
	s, err := scraper.NewScraper()
	if err != nil {
		return nil, err
	}

	return &ScrapeController{
		minio:   minio,
		scraper: s,
	}, nil
}

// Close releases resources used by the scraper
func (c *ScrapeController) Close() {
	if c.scraper != nil {
		c.scraper.Close()
	}
}

// Scrape performs the web scraping and stores the result
func (c *ScrapeController) Scrape(ctx context.Context, userID int, req types.ScrapeRequest) (*types.ScrapeResponse, error) {
	if req.URL == "" {
		return nil, nil
	}

	// Scrape the page
	result, err := c.scraper.ScrapePage(ctx, req.URL, req.WordLimit)
	if err != nil {
		return nil, err
	}

	// Upload to storage
	key, err := c.minio.UploadScrape(ctx, req.URL, result, "")
	if err != nil {
		return nil, err
	}

	return &types.ScrapeResponse{
		Key:     key,
		URL:     req.URL,
		Data:    result[:req.WordLimit],
		Message: "Scraped and stored successfully",
	}, nil
}

// // QueryWeb retrieves scraped results for queries
// func (c *ScrapeController) QueryWeb(queries []string, userID int) ([]any, error) {
// 	return c.scraper.QueryWeb(ctx, userID, sessionID)
// }

func (c *ScrapeController) QueryWebMulti(queries []string, limit int) (any, error) {
	queryResults := map[string]interface{}{}
	for _, u := range queries {
		text, _ := c.scraper.QueryWeb(u, limit)
		queryResults[u] = text
	}
	return queryResults, nil
}
