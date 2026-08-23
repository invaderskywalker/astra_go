package state

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ChatMessage is the file-backed conversation record used by the CLI agent.
// It deliberately lives beside the session manifest so a session is portable
// and inspectable without a database.
type ChatMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func AppendChatMessage(root, sessionID, role, content string) error {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	path := SessionHistoryPath(root, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(ChatMessage{Role: role, Content: content, Timestamp: time.Now().UTC()})
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}

func ReadChatHistory(root, sessionID string) ([]map[string]string, error) {
	file, err := os.Open(SessionHistoryPath(root, sessionID))
	if os.IsNotExist(err) {
		return []map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := []map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var message ChatMessage
		if json.Unmarshal(scanner.Bytes(), &message) != nil || strings.TrimSpace(message.Content) == "" {
			continue
		}
		result = append(result, map[string]string{"role": message.Role, "content": message.Content})
	}
	return result, scanner.Err()
}
