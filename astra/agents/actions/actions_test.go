package actions

import (
	"testing"

	"astra/astra/utils/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// --- Helpers ---
func setupTestEnv(t *testing.T) *DataActions {

	logging.InitLogger() // ensures AppLogger isn’t nil
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	return NewDataActions(db, 1)
}

// --- Scraping Actions Test: ScrapeURLs ---
func TestScrapeURLsAction(t *testing.T) {
	a := setupTestEnv(t)
	params := map[string]interface{}{
		"urls": []string{"https://example.com", "https://www.wikipedia.org"},
	}
	result, err := a.ExecuteAction("scrape_urls", params)
	if err != nil {
		t.Errorf("scrape_urls action failed: %v", err)
	}
	results, ok := result["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Errorf("expected non-empty results from scrape_urls")
	}
}

// --- Scraping Actions Test: QueryWeb ---
func TestQueryWebAction(t *testing.T) {

	a := setupTestEnv(t)
	params := map[string]interface{}{
		"queries":      []string{"openai gpt"},
		"result_limit": 2,
	}
	result, err := a.ExecuteAction("query_web", params)
	if err != nil {
		t.Errorf("query_web action failed: %v", err)
	}
	results, ok := result["results"].(map[string]interface{})
	if !ok || len(results) == 0 {
		t.Errorf("expected non-empty results from query_web")
	}
}

func TestQueryWebAction_MultiQuery(t *testing.T) {
	a := setupTestEnv(t)
	params := map[string]interface{}{
		"queries":      []string{"openai gpt", "github copilot", "chatbot ai"},
		"result_limit": 2,
	}
	result, err := a.ExecuteAction("query_web", params)
	if err != nil {
		t.Errorf("query_web (multi) action failed: %v", err)
	}
	results, ok := result["results"].(map[string]interface{})
	if !ok {
		t.Errorf("expected map of results from query_web (multi)")
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results for 3 queries, got %d", len(results))
	}
	for _, q := range []string{"openai gpt", "github copilot", "chatbot ai"} {
		if _, present := results[q]; !present {
			t.Errorf("missing result for query '%s'", q)
		}
	}
}
