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

	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")

	if dbHost == "" {
		panic("DB_HOST is a required environment variable")
	}

	if dbUser == "" {
		panic("DB_USER is a required environment variable")
	}

	DB, err = gorm.Open("postgres",
		fmt.Sprintf("host=%s sslmode=disable dbname=pion user=%s password=%s", dbHost, dbUser, dbPass))

	if err != nil {
		panic(err)
	}
}
