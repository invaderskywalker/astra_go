// astra/sources/psql/dao/dao.note.go
package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NoteDAO struct {
	DB *gorm.DB
}

func NewNoteDAO(db *gorm.DB) *NoteDAO {
	return &NoteDAO{DB: db}
}

func (dao *NoteDAO) CreateNote(ctx context.Context, note *models.Note) error {
	return dao.DB.WithContext(ctx).Create(note).Error
}

func (dao *NoteDAO) GetNoteByID(ctx context.Context, id uuid.UUID) (*models.Note, error) {
	var note models.Note
	err := dao.DB.WithContext(ctx).First(&note, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func (dao *NoteDAO) GetAllNotesByUser(ctx context.Context, userID int) ([]models.Note, error) {
	var notes []models.Note
	err := dao.DB.WithContext(ctx).Where("user_id = ?", userID).Order("updated_at desc").Find(&notes).Error
	if err != nil {
		return nil, err
	}
	return notes, nil
}

func (dao *NoteDAO) UpdateNote(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return dao.DB.WithContext(ctx).Model(&models.Note{}).Where("id = ?", id).Updates(updates).Error
}

func (dao *NoteDAO) DeleteNote(ctx context.Context, id uuid.UUID) error {
	return dao.DB.WithContext(ctx).Where("id = ?", id).Delete(&models.Note{}).Error
}
