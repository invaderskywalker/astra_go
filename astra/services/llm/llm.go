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
	"strings"

	"go.uber.org/zap"
)

type OllamaClient struct {
	baseURL string
}

func NewOllamaClient() *OllamaClient {
	return &OllamaClient{baseURL: "http://localhost:11434/api"}
}

type LLMClient interface {
	Run(ctx context.Context, req ChatRequest) (string, error)
	RunStream(ctx context.Context, req ChatRequest) (<-chan string, error)
}

func NewClient(provider string) LLMClient {
	switch provider {
	case "gpt", "openai":
		return NewGPTClient()
	case "ollama":
		return NewOllamaClient()
	default:
		logging.AppLogger.Warn("Unknown LLM provider, defaulting to Ollama", zap.String("provider", provider))
		return NewOllamaClient()
	}
}

type ChatRequest struct {
	Model    string      `json:"model"`
	Messages []Message   `json:"messages"`
	Stream   bool        `json:"stream"`
	Options  interface{} `json:"options,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
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
	return resp.Message.Content, nil
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

			if chunk.Message.Content != "" {
				select {
				case ch <- chunk.Message.Content:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}
