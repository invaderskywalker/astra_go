package scraper

import (
	"astra/astra/utils/types"
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
	"golang.org/x/net/html"
)

// Scraper struct to manage Playwright browser context
type Scraper struct {
	pw *playwright.Playwright
}

// NewScraper initializes Playwright
func NewScraper() (*Scraper, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	return &Scraper{pw: pw}, nil
}

// Close stops Playwright
func (s *Scraper) Close() {
	if s.pw != nil {
		s.pw.Stop()
	}
}

// ScrapePage scrapes a single URL and returns text content (maxChars limit)
func (s *Scraper) ScrapePage(ctx context.Context, targetURL string, opts *types.ScrapeOptions) (string, error) {
	// Apply defaults if nil/zero
	if opts == nil {
		opts = &types.ScrapeOptions{} // Zero values
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = 1000 // Default limit
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}

	browser, err := s.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		return "", err
	}
	defer browser.Close()

	context, err := browser.NewContext()
	if err != nil {
		return "", err
	}
	page, err := context.NewPage()
	if err != nil {
		return "", err
	}

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	}
	page.SetExtraHTTPHeaders(map[string]string{"User-Agent": userAgents[time.Now().UnixNano()%int64(len(userAgents))]})

	if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
		Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
		// Timeout:   playwright.Float(15000),
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return "", err
	}

	content, err := page.Content()
	if err != nil {
		return "", err
	}

	text := extractText(content)
	// if len(text) > maxChars {
	// 	text = text
	// }
	return text, nil
}

// extractText extracts text content from HTML
func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data + " ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	text := sb.String()
	// remove common phrases
	redundant := []string{"home", "contact us", "about us", "privacy policy", "terms of service"}
	for _, r := range redundant {
		text = strings.ReplaceAll(strings.ToLower(text), r, "")
	}
	return strings.TrimSpace(text)
}

func (s *Scraper) QueryWeb(query string, maxResults int) ([]types.SearchResult, error) {
	searchURL := "https://duckduckgo.com/html/"
	client := &http.Client{}
	params := url.Values{}
	params.Add("q", query)
	req, _ := http.NewRequest("GET", searchURL+"?"+params.Encode(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []types.SearchResult
	doc.Find(".result__body").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if i >= maxResults {
			return false
		}
		titleSel := s.Find(".result__title a")
		snippetSel := s.Find(".result__snippet")
		if titleSel.Length() == 0 || snippetSel.Length() == 0 {
			return true
		}

		href, exists := titleSel.Attr("href")
		if !exists {
			return true
		}

		parsed, _ := url.Parse(href)
		actualURL := parsed.Query().Get("uddg")
		if actualURL == "" || !regexp.MustCompile(`^https?://`).MatchString(actualURL) {
			return true
		}

		results = append(results, types.SearchResult{
			URL:     actualURL,
			Title:   titleSel.Text(),
			Snippet: snippetSel.Text(),
		})
		return true
	})

	return results, nil
}
