// astra/utils/types/chat.go
package types

type ChatRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
}

// For session/thread summary in threads panel
// LastActivity: RFC3339 string
type ChatSessionSummary struct {
	SessionID       string `json:"session_id"`
	LastMessage     string `json:"last_message"`
	LastMessageRole string `json:"last_message_role"`
	LastActivity    string `json:"last_activity"`
}
