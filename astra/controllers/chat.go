// astra/controllers/chat.go
package controllers

import (
	"astra/astra/sources/psql/dao"
	httputils "astra/astra/utils/http"
	"astra/astra/utils/types"
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
		"model":    "llama3:8b",
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
		"model":    "llama3:8b",
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
				ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := c.chatDAO.SaveMessage(ctx2, sessionID, userID, "assistant", fullContent)
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

// NEW: List all chat sessions (threads) for a user, sorted by last activity.
func (c *ChatController) ListSessions(ctx context.Context, userID int) ([]types.ChatSessionSummary, error) {
	return c.chatDAO.ListSessionsForUser(ctx, userID)
}

// NEW: Get all messages for a session (ownership enforced)
func (c *ChatController) GetMessagesForSession(ctx context.Context, userID int, sessionID string) ([]map[string]interface{}, error) {
	// Security: check this session is owned by the user
	belongs, err := c.chatDAO.SessionBelongsToUser(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, errors.New("session not found or forbidden")
	}
	msgs, err := c.chatDAO.GetMessagesBySession(ctx, sessionID, userID)
	if err != nil {
		return nil, err
	}
	// Convert to API shape (role, content, timestamp, id)
	result := make([]map[string]interface{}, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]interface{}{
			"id":        m.ID,
			"role":      m.Role,
			"content":   m.Content,
			"timestamp": m.Timestamp.Format(time.RFC3339),
		}
	}
	return result, nil
}
