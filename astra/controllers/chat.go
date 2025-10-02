// astra/controllers/chat.go (new)
package controllers

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/types"
	httputils "astra/astra/utils/http"
	"bufio"
	"context"
	"encoding/json"
	"time"
)

type ChatController struct {
	chatDAO *dao.ChatMessageDAO
}

func NewChatController(chatDAO *dao.ChatMessageDAO) *ChatController {
	return &ChatController{chatDAO: chatDAO}
}

func (c *ChatController) Chat(ctx context.Context, userID int, req types.ChatRequest) (map[string]string, error) {
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = c.chatDAO.CreateSessionID()
	}
	_, err := c.chatDAO.SaveMessage(ctx, sessionID, userID, "user", req.Content)
	if err != nil {
		return nil, err
	}
	history, err := c.chatDAO.GetChatHistoryBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	llmReq := map[string]interface{}{
		"model":    "gpt-oss:120b-cloud",
		"messages": history,
		"stream":   false,
	}
	var llmResp struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	}
	err = httputils.PostJSON("http://localhost:11434/api/chat", llmReq, &llmResp)
	if err != nil {
		return nil, err
	}
	content := llmResp.Message.Content
	_, err = c.chatDAO.SaveMessage(ctx, sessionID, userID, "assistant", content)
	if err != nil {
		return nil, err
	}
	return map[string]string{"response": content, "session_id": sessionID}, nil
}

func (c *ChatController) ChatStream(ctx context.Context, userID int, req types.ChatRequest) (chan string, chan error) {
	errCh := make(chan error, 1)
	ch := make(chan string)

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = c.chatDAO.CreateSessionID()
	}
	_, err := c.chatDAO.SaveMessage(ctx, sessionID, userID, "user", req.Content)
	if err != nil {
		errCh <- err
		close(ch)
		close(errCh)
		return ch, errCh
	}
	history, err := c.chatDAO.GetChatHistoryBySession(ctx, sessionID)
	if err != nil {
		errCh <- err
		close(ch)
		close(errCh)
		return ch, errCh
	}
	llmReq := map[string]interface{}{
		"model":    "gpt-oss:120b-cloud",
		"messages": history,
		"stream":   true,
	}
	body, err := httputils.PostStream("http://localhost:11434/api/chat", llmReq)
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
		scanner := bufio.NewScanner(body)
		var fullContent string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				errCh <- err
				return
			}
			if chunk.Done {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := c.chatDAO.SaveMessage(ctx, sessionID, userID, "assistant", fullContent)
				if err != nil {
					errCh <- err
				}
				return
			}
			delta := chunk.Message.Content
			fullContent += delta
			ch <- delta
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
		}
	}()

	return ch, errCh
}
