// astra/types/scrape.go
package types

import (
	"time"
)

type ScrapeOptions struct {
	MaxChars int
	Timeout  time.Duration // e.g., default 15s
}

type ScrapeRequest struct {
	URLs      []string `json:"urls"`
	WordLimit *int     `json:"word_limit,omitempty"` // Pointer: nil means "use default"
}

type ScrapeResult struct {
	URL     string `json:"url"`
	Content string `json:"content"`
	Error   string `json:"error"`
}

type SearchResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type QueryWebRequest struct {
	Queries     []string `json:"queries"`
	ResultLimit int      `json:"result_limit"`
}

type QueryWebResponse struct {
}

type ScrapeResponse struct {
	Key     string `json:"key"`
	URL     string `json:"url"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}
