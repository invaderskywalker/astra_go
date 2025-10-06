// astra/services/llm/groq_client.go
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

type GroqClient struct {
	baseURL string
	apiKey  string
}

// NewGroqClient returns a client pointing to the Groq Chat endpoint.
// You may provide your API key via env var or config.
func NewGroqClient(apiKey string) *GroqClient {
	// Groq’s OpenAI-compatible base path is usually: https://api.groq.com/openai/v1
	return &GroqClient{
		baseURL: "https://api.groq.com/openai/v1",
		apiKey:  apiKey,
	}
}

// Run (non-streaming) chat completion
func (c *GroqClient) Run(ctx context.Context, req ChatRequest) (string, error) {
	defer logging.LogDuration(ctx, "groq_service_run")()

	// Build full URL
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)

	// Prepare container corresponding to OpenAI-style response
	var resp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}

	// Use your HTTP util, but ensure it sets Authorization header
	if err := httputils.PostJSONWithAuth(url, c.apiKey, req, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("no choices returned")
}

// RunStream — streaming version using SSE / chunked responses
func (c *GroqClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	fmt.Println("groq_service_run_stream")
	defer logging.LogDuration(ctx, "groq_service_run_stream")()

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	body, err := httputils.PostStreamWithAuth(url, c.apiKey, req)
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
				logging.AppLogger.Info("Groq RunStream context cancelled")
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				logging.ErrorLogger.Error("groq stream read error", zap.Any("err", err))
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Groq uses SSE “data:” prefix in streaming mode
			if strings.HasPrefix(line, "data:") {
				line = strings.TrimPrefix(line, "data:")
				line = strings.TrimSpace(line)
			}

			if line == "[DONE]" {
				return
			}

			// Parse the JSON chunk
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message *Message `json:"message,omitempty"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				logging.ErrorLogger.Error("groq stream JSON parse error",
					zap.Any("err", err), zap.String("raw_line", line))
				continue
			}

			// Send deltas or messages
			for _, choice := range chunk.Choices {
				// prefer delta
				if choice.Delta.Content != "" {
					select {
					case ch <- choice.Delta.Content:
					case <-ctx.Done():
						return
					}
				} else if choice.Message != nil && choice.Message.Content != "" {
					select {
					case ch <- choice.Message.Content:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}
