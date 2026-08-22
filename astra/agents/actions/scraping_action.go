package actions

import (
	"astra/astra/services/scraper"
	"fmt"
)

type ScrapeURLsParams struct {
	URLs      []string `json:"urls"`
	WordLimit *int     `json:"word_limit,omitempty"`
}

type QueryWebParams struct {
	Queries     []string `json:"queries"`
	ResultLimit int      `json:"result_limit"`
}

// Action to scrape given URLs and return their text contents
func (a *DataActions) ScrapeURLs(params ScrapeURLsParams) ActionResult {
	s, err := scraper.NewScraper()
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("scraper init error: %v", err),
		}
	}
	defer s.Close()

	results, err := s.ReadMultiplePages(params.URLs, 2)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("scrape error: %v", err),
		}
	}
	filesRead := make([]string, len(params.URLs))
	copy(filesRead, params.URLs)
	return ActionResult{
		Success:     true,
		Summary:     fmt.Sprintf("Scraped %d URL(s) successfully", len(results)),
		Diagnostics: results,
		FilesRead:   filesRead,
	}
}

// Action to perform web search queries and return the text snippets
func (a *DataActions) QueryWeb(params QueryWebParams) ActionResult {
	s, err := scraper.NewScraper()
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("scraper init error: %v", err),
		}
	}
	defer s.Close()

	queryResults := map[string]interface{}{}
	for _, u := range params.Queries {
		text, _ := s.QueryWeb(u, params.ResultLimit)
		queryResults[u] = text
	}

	return ActionResult{
		Success:     true,
		Summary:     fmt.Sprintf("Fetched %d query result(s)", len(queryResults)),
		Diagnostics: queryResults,
	}
}
