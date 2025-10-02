package dao

import (
	"astra/astra/sources/psql/models"
	"context"

	"gorm.io/gorm"
)

type UserDAO struct {
	DB *gorm.DB
}

func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{DB: db}
}

func (dao *UserDAO) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User
	err := dao.DB.WithContext(ctx).First(&user, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil // Consistent with original
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *UserDAO) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := dao.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *UserDAO) CreateUser(ctx context.Context, username, email string, fullName *string) (*models.User, error) {
	user := models.User{
		Username: username,
		Email:    email,
		FullName: fullName,
	}
	err := dao.DB.WithContext(ctx).Create(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (dao *UserDAO) GetAllUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := dao.DB.WithContext(ctx).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}
