// astra/services/llm/llm.go
package llm

import (
	httputils "astra/astra/utils/http"
	"astra/astra/utils/logging"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
)

type OllamaClient struct {
	baseURL string
}

func NewOllamaClient() *OllamaClient {
	baseURL := strings.TrimSuffix(os.Getenv("OLLAMA_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434/api"
	}
	return &OllamaClient{baseURL: baseURL}
}

type LLMClient interface {
	Run(ctx context.Context, req ChatRequest) (string, error)
	RunStream(ctx context.Context, req ChatRequest) (<-chan string, error)
}

func NewClient(provider string) LLMClient {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gpt", "openai":
		return NewGPTClient()
	case "ollama":
		return NewOllamaClient()
	default:
		logging.AppLogger.Warn("Unknown LLM provider, defaulting to Ollama", zap.String("provider", provider))
		return NewOllamaClient()
	}
}

func DefaultModel(provider string) string {
	if model := strings.TrimSpace(os.Getenv("ASTRA_LLM_MODEL")); model != "" {
		return model
	}
	if strings.EqualFold(provider, "ollama") {
		return "qwen3:14b"
	}
	return "gpt-5.6-luna"
}

func DefaultProvider() string {
	if provider := strings.TrimSpace(os.Getenv("ASTRA_LLM_PROVIDER")); provider != "" {
		return provider
	}
	return "ollama"
}

// ListOllamaModels returns installed local model names without requiring an LLM request.
func ListOllamaModels(ctx context.Context) ([]string, error) {
	baseURL := strings.TrimSuffix(os.Getenv("OLLAMA_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:11434/api"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/tags", nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama model list failed: %s", response.Status)
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(payload.Models))
	for _, model := range payload.Models {
		models = append(models, model.Name)
	}
	return models, nil
}

type ChatRequest struct {
	Model    string      `json:"model"`
	Messages []Message   `json:"messages"`
	Stream   bool        `json:"stream"`
	Options  interface{} `json:"options,omitempty"`
}

// type Message struct {
// 	Role    string `json:"role"`
// 	Content string `json:"content"`
// }

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}
type ChatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

func extractTextContent(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v

	case []interface{}:
		// OpenAI style multimodal / block content
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				if t, ok := m["text"].(string); ok {
					sb.WriteString(t)
				}
			}
		}
		return sb.String()

	case map[string]interface{}:
		// Defensive: sometimes models wrap content
		if t, ok := v["text"].(string); ok {
			return t
		}
	}

	return ""
}

// -----------------------------
// Non-stream (regular) Run
// -----------------------------
func (c *OllamaClient) Run(ctx context.Context, req ChatRequest) (string, error) {
	defer logging.LogDuration(ctx, "llm_service_run")()

	var resp ChatResponse
	if err := httputils.PostJSON(c.baseURL+"/chat", req, &resp); err != nil {
		return "", err
	}
	return extractTextContent(resp.Message.Content), nil
	// return resp.Message.Content, nil
}

// -----------------------------
// Fixed Streaming Version
// -----------------------------
func (c *OllamaClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	fmt.Println("llm_service_run_stream (ollama)")
	defer logging.LogDuration(ctx, "llm_service_run_stream")()

	body, err := httputils.PostStream(c.baseURL+"/chat", req)
	if err != nil {
		return nil, err
	}

	ch := make(chan string)

	go func() {
		defer func() {
			close(ch)
			body.Close()
		}()

		reader := bufio.NewReader(body)

		for {
			select {
			case <-ctx.Done():
				logging.AppLogger.Info("llm RunStream context cancelled")
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				logging.ErrorLogger.Error("llm stream read error", zap.Any("err", err))
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Ollama sends raw JSON per line, or sometimes "data: {...}"
			if strings.HasPrefix(line, "data:") {
				line = strings.TrimPrefix(line, "data:")
				line = strings.TrimSpace(line)
			}

			if line == "[DONE]" {
				return
			}

			var chunk ChatResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				logging.ErrorLogger.Error("llm stream JSON parse error",
					zap.Any("err", err), zap.String("raw_line", line))
				continue
			}

			if chunk.Done {
				return
			}

			// if chunk.Message.Content != "" {
			// 	select {
			// 	case ch <- chunk.Message.Content:
			// 	case <-ctx.Done():
			// 		return
			// 	}
			// }
			text := extractTextContent(chunk.Message.Content)
			if text != "" {
				select {
				case ch <- text:
				case <-ctx.Done():
					return
				}
			}

		}
	}()

	return ch, nil
}
