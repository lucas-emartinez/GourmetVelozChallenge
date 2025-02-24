package database

import (
	"YunoChallenge/internal/platform/config"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

// Database DB is a struct that embeds a sql.DB
type Database struct {
	*sql.DB
}

// New creates a connection to the database and returns a reference to the DB struct
func New(config config.DatabaseConfig) (*Database, error) {
	dsn := config.User + ":" + config.Pass + "@tcp(" + config.Host + ":" + config.Port + ")/?parseTime=true"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS YunoChallenge")
	if err != nil {
		db.Close()
		return nil, err
	}

	_, err = db.Exec("USE YunoChallenge")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to use database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	// Create the orders table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			arrival_time DATETIME NOT NULL,
			dishes TEXT NOT NULL,
			status ENUM('pending', 'preparing', 'ready', 'delivered', 'cancelled') NOT NULL,
			source VARCHAR(50) NOT NULL,
			vip BOOLEAN NOT NULL DEFAULT FALSE,
			preparation_time TIME DEFAULT NULL
		)
	`)

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	log.Println("Database connected and initialized")
	return &Database{db}, nil
}

// Close closes the database connection
func (db *Database) Close() error {
	return db.DB.Close()
}
