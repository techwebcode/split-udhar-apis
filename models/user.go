package models

type User struct {
	BaseModel
	FullName   string `gorm:"size:100;not null" json:"full_name"`
	Email      string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Mobile     string `gorm:"size:15;uniqueIndex;not null" json:"mobile"`
	IsVerified bool   `gorm:"default:false" json:"is_verified"`
	MPIN       string `gorm:"size:20" json:"-"`
}
