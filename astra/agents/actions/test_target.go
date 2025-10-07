package actions

import (
	"astra/astra/sources/psql/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// test_target.go — intentionally simple and editable file for code edit testing

func DemoFunction() {
	fmt.Println("start")
	fmt.Println("middle")
	fmt.Println("end")
}

type DemoStruct struct {
	Name string
}

func UnusedFunction() {
	fmt.Println("this should be replaced")
}

type User struct {
	ID       int     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string  `json:"username" gorm:"type:varchar(255);not null"`
	Email    string  `json:"email" gorm:"type:varchar(255);not null"`
	FullName *string `json:"full_name,omitempty" gorm:"type:varchar(255)"`
}

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
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
