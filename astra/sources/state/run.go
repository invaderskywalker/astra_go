package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AppendRunEvent writes a local, per-run evidence stream. Session-level
// history remains available separately; this file makes it possible to inspect
// exactly what happened for one user submission without filtering a large
// session transcript.
func AppendRunEvent(root, sessionID, runID, eventType string, payload any) error {
	if strings.TrimSpace(runID) == "" {
		return nil
	}
	event := map[string]any{
		"timestamp":  time.Now().UTC(),
		"session_id": sessionID,
		"run_id":     runID,
		"type":       eventType,
		"payload":    payload,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	path := RunEventsPath(root, sessionID, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
