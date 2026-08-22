// astra/types/scrape.go
package types

import (
	"time"
)

type ScrapeOptions struct {
	MaxChars int
	Timeout  time.Duration // e.g., default 15s
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
