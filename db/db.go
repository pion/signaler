package db

import (
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Postgres driver for gorm
)

var (
	DB *gorm.DB // Pion global database
)

func init() {
	var err error

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")

	if dbUser == "" {
		panic("DB_USER is a required environment variable")
	}

	DB, err = gorm.Open("postgres",
		fmt.Sprintf("host=db sslmode=disable dbname=pion user=%s password=%s", dbUser, dbPass))

	if err != nil {
		panic(err)
	}
}
