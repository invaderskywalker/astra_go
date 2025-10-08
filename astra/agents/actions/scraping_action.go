package actions

import (
	"astra/astra/services/scraper"
	"astra/astra/utils/types"
)

type ScrapeURLsParams struct {
	URLs      []string `json:"urls"`
	WordLimit *int     `json:"word_limit,omitempty"`
}

type ScrapeURLsResult struct {
	Results []types.ScrapeResult `json:"results"`
}

type QueryWebParams struct {
	Queries     []string `json:"queries"`
	ResultLimit int      `json:"result_limit"`
}

type QueryWebResult struct {
	Results map[string]interface{} `json:"results"`
}

// Action to scrape given URLs and return their text contents
func (a *DataActions) ScrapeURLs(params ScrapeURLsParams) (ScrapeURLsResult, error) {
	s, err := scraper.NewScraper()
	if err != nil {
		return ScrapeURLsResult{}, err
	}
	defer s.Close()

	results, err := s.ReadMultiplePages(params.URLs, 2)
	if err != nil {
		return ScrapeURLsResult{}, err
	}
	return ScrapeURLsResult{Results: results}, nil
}

// Action to perform web search queries and return the text snippets
func (a *DataActions) QueryWeb(params QueryWebParams) (QueryWebResult, error) {
	s, err := scraper.NewScraper()
	if err != nil {
		return QueryWebResult{}, err
	}
	defer s.Close()
	queryResults := map[string]interface{}{}
	for _, u := range params.Queries {
		text, _ := s.QueryWeb(u, params.ResultLimit)
		queryResults[u] = text
	}
	return QueryWebResult{Results: queryResults}, nil
}
