// astra/types/chat_request.go (new)
package types

type ChatRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
}
