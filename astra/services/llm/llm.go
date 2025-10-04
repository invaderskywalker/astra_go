// astra/services/llm/llm.go (new)
package llm

import (
	httputils "astra/astra/utils/http"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.uber.org/zap"
)

type OllamaClient struct {
	baseURL string
}

func NewOllamaClient() *OllamaClient {
	return &OllamaClient{baseURL: "http://localhost:11434/api"}
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

func (c *OllamaClient) Run(ctx context.Context, req ChatRequest) (string, error) {
	defer logging.LogDuration(ctx, "llm_service_run")()
	var resp ChatResponse
	if err := httputils.PostJSON(c.baseURL+"/chat", req, &resp); err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

// Replace the existing RunStream with this version.

func (c *OllamaClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	fmt.Println("llm_service_run_stream")
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

		decoder := json.NewDecoder(body)

		for {
			// If caller cancelled context, stop reading.
			select {
			case <-ctx.Done():
				logging.AppLogger.Info("llm RunStream context cancelled")
				return
			default:
				// continue
			}

			var chunk ChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					// normal EOF -> finish cleanly
					return
				}
				// unexpected decode error: log and exit
				logging.ErrorLogger.Error("llm stream decode error", zap.Any("err", err))
				return
			}
			// If server signals done, finish.
			if chunk.Done {
				return
			}
			// send content (may be empty) — non-blocking send to avoid goroutine leak
			select {
			case ch <- chunk.Message.Content:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}
