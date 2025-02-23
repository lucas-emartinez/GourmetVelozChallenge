package database

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// Database DB is a struct that embeds a sql.DB
type Database struct {
	*sql.DB
}

// New creates a connection to the database and returns a reference to the DB struct
func New(dsn string) (*Database, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	log.Println("Database connected")
	return &Database{db}, nil
}

// GetDB returns the database connection
func (db *Database) GetDB() *sql.DB {
	return db.DB
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.DB.Close()
}
