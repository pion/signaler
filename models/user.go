package models

import (
	"github.com/jinzhu/gorm"
)

// User base user definition contains `Name`, `Email` and `APIKey`
type User struct {
	gorm.Model

	Name    string
	Email   string
	APIKey  string `gorm:"type:varchar(36)"`
	MaxKeys int
}
