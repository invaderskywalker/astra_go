package scraper

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

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
func (s *Scraper) ScrapePage(ctx context.Context, targetURL string, maxChars int) (string, error) {
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
		Timeout:   playwright.Float(15000),
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

func (s *Scraper) QueryWeb(query string, maxResults int) ([]string, error) {
	searchURL := "https://duckduckgo.com/html/"
	client := &http.Client{Timeout: 10 * time.Second}

	params := url.Values{}
	params.Add("q", query)
	req, _ := http.NewRequest("GET", searchURL+"?"+params.Encode(), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var urls []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, "uddg=") {
					u, _ := url.Parse(attr.Val)
					q := u.Query()
					if actual := q.Get("uddg"); actual != "" {
						if matched, _ := regexp.MatchString(`^https?://[\w\-\.]+`, actual); matched {
							urls = append(urls, actual)
							if len(urls) >= maxResults {
								return
							}
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return urls, nil
}
