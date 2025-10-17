// astra/sources/psql/dao/dao.session_summary.go
package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"gorm.io/gorm"
)

type SessionSummaryDAO struct {
	DB *gorm.DB
}

func NewSessionSummaryDAO(db *gorm.DB) *SessionSummaryDAO {
	return &SessionSummaryDAO{DB: db}
}

// UpsertSessionSummary creates or updates a session summary for a given session and user.
func (dao *SessionSummaryDAO) UpsertSessionSummary(ctx context.Context, sessionID string, userID int, summary string) (*models.SessionSummary, error) {
	var ss models.SessionSummary
	err := dao.DB.WithContext(ctx).
		Where("session_id = ? AND user_id = ?", sessionID, userID).
		First(&ss).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			newSS := models.SessionSummary{
				SessionID: sessionID,
				UserID:    userID,
				Summary:   summary,
			}
			if err := dao.DB.WithContext(ctx).Create(&newSS).Error; err != nil {
				return nil, err
			}
			return &newSS, nil
		}
		return nil, err
	}
	ss.Summary = summary
	if err := dao.DB.WithContext(ctx).Save(&ss).Error; err != nil {
		return nil, err
	}
	return &ss, nil
}

// GetSessionSummaryBySessionID retrieves a session summary for a given session and user.
func (dao *SessionSummaryDAO) GetSessionSummaryBySessionID(ctx context.Context, sessionID string, userID int) (*models.SessionSummary, error) {
	var ss models.SessionSummary
	err := dao.DB.WithContext(ctx).
		Where("session_id = ? AND user_id = ?", sessionID, userID).
		First(&ss).Error
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

// DeleteSessionSummaryBySessionID deletes a session summary for a given session and user.
func (dao *SessionSummaryDAO) DeleteSessionSummaryBySessionID(ctx context.Context, sessionID string, userID int) error {
	return dao.DB.WithContext(ctx).
		Where("session_id = ? AND user_id = ?", sessionID, userID).
		Delete(&models.SessionSummary{}).Error
}

// ListRecentSessionSummaries returns up to N most recent session summaries for a user, ordered by updated_at DESC.
func (dao *SessionSummaryDAO) ListRecentSessionSummaries(ctx context.Context, userID int, limit int) ([]models.SessionSummary, error) {
	var summaries []models.SessionSummary
	err := dao.DB.WithContext(ctx).
		Model(&models.SessionSummary{}).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&summaries).Error
	if err != nil {
		return nil, err
	}
	return summaries, nil
}
