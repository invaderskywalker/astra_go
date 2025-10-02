// astra/services/llm/llm.go (new)
package llm

import (
	httputils "astra/astra/utils/http"
	"context"
	"encoding/json"
	"io"
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
	var resp ChatResponse
	if err := httputils.PostJSON(c.baseURL+"/chat", req, &resp); err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

func (c *OllamaClient) RunStream(ctx context.Context, req ChatRequest) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error, 1)

	body, err := httputils.PostStream(c.baseURL+"/chat", req)
	if err != nil {
		errCh <- err
		close(ch)
		close(errCh)
		return ch, errCh
	}

	go func() {
		defer close(ch)
		defer close(errCh)
		defer body.Close()

		decoder := json.NewDecoder(body)
		for {
			var chunk ChatResponse
			if err := decoder.Decode(&chunk); err == io.EOF {
				break
			} else if err != nil {
				errCh <- err
				return
			}
			if chunk.Done {
				break
			}
			ch <- chunk.Message.Content
		}
	}()

	return ch, errCh
}
