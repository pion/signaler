package db

import (
	"errors"
	"os"
)

var ()

func init() error {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	// dbPass := os.Getenv("DB_PASS")

	if dbHost == "" {
		return errors.New("DB_HOST is a required environment variable")
	}
	if dbUser == "" {
		return errors.New("DB_USER is a required environment variable")
	}

	return nil
}
