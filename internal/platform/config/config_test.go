package config

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "3306")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASS", "password")
	t.Setenv("SERVER_PORT", "8080")

	config := LoadConfig()

	if config.Database.Host != "localhost" {
		t.Errorf("expected Database.Host to be 'localhost', got '%s'", config.Database.Host)
	}
	if config.Database.Port != "3306" {
		t.Errorf("expected Database.Port to be '3306', got '%s'", config.Database.Port)
	}
	if config.Database.User != "root" {
		t.Errorf("expected Database.User to be 'root', got '%s'", config.Database.User)
	}
	if config.Database.Pass != "password" {
		t.Errorf("expected Database.Pass to be 'password', got '%s'", config.Database.Pass)
	}
	if config.Server.Port != "8080" {
		t.Errorf("expected Server.Port to be '8080', got '%s'", config.Server.Port)
	}
}
