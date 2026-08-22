package controllers

import (
	"astra/astra/sources/mindpalace"
	"astra/astra/sources/storage"
	"context"
)

// MemoryController serves file-backed mind-palace records; it never reads learning data from PostgreSQL.
type MemoryController struct {
	root   string
	mirror *storage.MinIOClient
}

func NewMemoryController(root string, mirror *storage.MinIOClient) *MemoryController {
	return &MemoryController{root: root, mirror: mirror}
}
func (c *MemoryController) List(ctx context.Context, userID int, kind string) ([]mindpalace.Record, error) {
	_ = ctx
	return mindpalace.New(c.root, userID, "", c.mirror).List(kind)
}
