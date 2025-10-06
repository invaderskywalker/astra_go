package llm

import (
	"astra/astra/utils/logging"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
)

type GPTClient struct {
	apiKey  string
	baseURL string
}

func NewGPTClient() *GPTClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logging.ErrorLogger.Fatal("Missing OPENAI_API_KEY environment variable")
	}
	return &GPTClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/chat/completions",
	}
}

type gptChatRequest struct {
	Model    string      `json:"model"`
	Messages []Message   `json:"messages"`
	Stream   bool        `json:"stream"`
	Options  interface{} `json:"options,omitempty"`
}

type gptResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type gptStreamResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Run executes a single GPT completion request (non-streaming)
func (c *GPTClient) Run(ctx context.Context, req ChatRequest) (string, error) {
	defer logging.LogDuration(ctx, "gpt_service_run")()

	gptReq := gptChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
		Options:  req.Options,
	}

	// Manual POST because we need custom headers
	body, err := json.Marshal(gptReq)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GPT request failed: %s - %s", resp.Status, string(b))
	}

	var parsed gptResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("failed to decode GPT response: %w", err)
	}

	if len(parsed.Choices) > 0 {
		return parsed.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no content in GPT response")
}

// RunStream handles streaming responses
// RunStream handles streaming responses (OpenAI / Groq / compatible)
func (c *GPTClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	defer logging.LogDuration(ctx, "gpt_service_run_stream")()

	gptReq := gptChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   true,
		Options:  req.Options,
	}

	body, err := json.Marshal(gptReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GPT stream request failed: %s - %s", resp.Status, string(b))
	}

	ch := make(chan string)

	go func() {
		defer func() {
			close(ch)
			resp.Body.Close()
		}()

		reader := bufio.NewReader(resp.Body)

		for {
			select {
			case <-ctx.Done():
				logging.AppLogger.Info("GPT stream context cancelled")
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				logging.ErrorLogger.Error("GPT stream read error", zap.Any("err", err))
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Skip comments and non-data lines
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				return
			}

			var chunk gptStreamResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				logging.ErrorLogger.Error("GPT stream JSON parse error",
					zap.Any("err", err), zap.String("raw_line", data))
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					select {
					case ch <- choice.Delta.Content:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}
