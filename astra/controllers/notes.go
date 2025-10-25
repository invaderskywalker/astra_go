// astra/controllers/notes.go
package controllers

import (
	"astra/astra/sources/psql/dao"
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
)

type NotesController struct {
	dao *dao.NoteDAO
}

func NewNotesController(dao *dao.NoteDAO) *NotesController {
	return &NotesController{dao: dao}
}

func (c *NotesController) CreateNote(ctx context.Context, userID int, title string, content string, favourite bool) (*models.Note, error) {
	note := &models.Note{
		UserID:    userID,
		Title:     title,
		Content:   content,
		Favourite: favourite,
	}
	err := c.dao.CreateNote(ctx, note)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (c *NotesController) GetNoteByID(ctx context.Context, id uuid.UUID) (*models.Note, error) {
	return c.dao.GetNoteByID(ctx, id)
}

func (c *NotesController) GetAllNotesByUser(ctx context.Context, userID int) ([]models.Note, error) {
	return c.dao.GetAllNotesByUser(ctx, userID)
}

func (c *NotesController) UpdateNote(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return c.dao.UpdateNote(ctx, id, updates)
}

func (c *NotesController) DeleteNote(ctx context.Context, id uuid.UUID) error {
	return c.dao.DeleteNote(ctx, id)
}
