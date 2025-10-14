package models

type User struct {
	ID       int     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string  `json:"username" gorm:"type:varchar(255);not null"`
	Email    string  `json:"email" gorm:"type:varchar(255);not null"`
	FullName *string `json:"full_name,omitempty" gorm:"type:varchar(255)"`
	ImageURL *string `json:"image_url,omitempty" gorm:"type:varchar(512)"`
}
