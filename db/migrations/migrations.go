package migrations

import (
	"gitlab.com/pions/pion/signaler/db"
	"gitlab.com/pions/pion/signaler/models"
)

func init() {
	db.DB.AutoMigrate(&models.User{})
}
