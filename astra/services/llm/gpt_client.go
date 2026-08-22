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
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type GPTClient struct {
	apiKey  string
	baseURL string
}

func NewGPTClient() *GPTClient {
	// 1️⃣ First, try to load local .env (for dev runs)
	_ = godotenv.Load(".env")

	// 2️⃣ Then fallback to global Astra config if API key still missing
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		home, _ := os.UserHomeDir()
		envPath := filepath.Join(home, ".astra", ".astra.env")
		_ = godotenv.Load(envPath)
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &GPTClient{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1/responses",
	}
}

type responsesRequest struct {
	Model     string      `json:"model"`
	Input     []Message   `json:"input"`
	Stream    bool        `json:"stream"`
	Reasoning interface{} `json:"reasoning,omitempty"`
}

type responsesResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

type responsesStreamEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

// Run executes a single GPT completion request (non-streaming)
func (c *GPTClient) Run(ctx context.Context, req ChatRequest) (string, error) {
	defer logging.LogDuration(ctx, "gpt_service_run")()
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is required for the OpenAI provider")
	}

	gptReq := responsesRequest{
		Model: req.Model, Input: req.Messages, Stream: false, Reasoning: req.Options,
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
		return "", fmt.Errorf("OpenAI Responses request failed: %s - %s", resp.Status, string(b))
	}

	var parsed responsesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("failed to decode GPT response: %w", err)
	}

	var text strings.Builder
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" {
				text.WriteString(content.Text)
			}
		}
	}
	if text.Len() == 0 {
		return "", fmt.Errorf("no text content in OpenAI response")
	}
	return text.String(), nil
}

// RunStream handles streaming responses
// RunStream handles streaming responses (OpenAI / Groq / compatible)
func (c *GPTClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, error) {
	defer logging.LogDuration(ctx, "gpt_service_run_stream")()
	if c.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required for the OpenAI provider")
	}

	gptReq := responsesRequest{
		Model: req.Model, Input: req.Messages, Stream: true, Reasoning: req.Options,
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
		return nil, fmt.Errorf("OpenAI Responses stream request failed: %s - %s", resp.Status, string(b))
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

			var chunk responsesStreamEvent
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				logging.ErrorLogger.Error("GPT stream JSON parse error",
					zap.Any("err", err), zap.String("raw_line", data))
				continue
			}

			if chunk.Type == "response.output_text.delta" && chunk.Delta != "" {
				select {
				case ch <- chunk.Delta:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}
