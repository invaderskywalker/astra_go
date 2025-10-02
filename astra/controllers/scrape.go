// astra/controllers/scrape.go
package controllers

import (
	"astra/astra/sources/storage"
	"astra/astra/utils/scraper"
	"astra/astra/utils/types"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
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
	var finalResults []types.ScrapeResult
	var urlsToScrape []string
	urlToIndex := make(map[string]int) // Map to preserve order

	// Step 1: Check MinIO for existing scrape results
	for i, url := range req.URLs {
		key := fmt.Sprintf("scrapes/%x.json", md5.Sum([]byte(url))) // same hashing logic
		data, err := c.minio.GetScrape(ctx, key)
		if err != nil {
			// If error is object not found → mark for scraping
			urlsToScrape = append(urlsToScrape, url)
			urlToIndex[url] = i
			finalResults = append(finalResults, types.ScrapeResult{URL: url}) // placeholder
			continue
		}

		// If exists → unmarshal and add to finalResults
		var obj storage.ScrapeObject
		if err := json.Unmarshal([]byte(data), &obj); err != nil {
			// If unmarshalling fails, scrape again
			urlsToScrape = append(urlsToScrape, url)
			urlToIndex[url] = i
			finalResults = append(finalResults, types.ScrapeResult{URL: url}) // placeholder
			continue
		}

		finalResults = append(finalResults, types.ScrapeResult{
			URL:     obj.URL,
			Content: obj.Text,
			Error:   "",
		})
	}

	// Step 2: Scrape only missing URLs
	if len(urlsToScrape) > 0 {
		scrapeResults, err := c.scraper.ReadMultiplePages(urlsToScrape, 2)
		if err != nil {
			return nil, err
		}

		// Step 3: Upload newly scraped results to MinIO and insert into finalResults
		for _, res := range scrapeResults {
			key, err := c.minio.UploadScrape(ctx, res.URL, res.Content, "")
			if err != nil {
				res.Error = err.Error()
			} else {
				fmt.Println("Uploaded scrape result to MinIO:", key)
			}

			// Insert result at correct index
			index := urlToIndex[res.URL]
			finalResults[index] = res
		}
	}

	return &types.ScrapeResponse{
		Message: "Scraped and stored successfully",
		// You can also include results in the response if needed:
		Data: finalResults,
	}, nil
}

func (c *ScrapeController) QueryWebMulti(queries []string, limit int) (any, error) {
	queryResults := map[string]interface{}{}
	for _, u := range queries {
		text, _ := c.scraper.QueryWeb(u, limit)
		queryResults[u] = text
	}
	return queryResults, nil
}
