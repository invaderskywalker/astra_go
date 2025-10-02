// astra/types/scrape.go
package types

type ScrapeRequest struct {
	URL       string `json:"url"`
	WordLimit int    `json:"word_limit"`
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
	Data    string `json:"data"`
}
