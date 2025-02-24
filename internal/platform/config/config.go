package config

// Config holds the configuration for the application
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
}

// DatabaseConfig holds the configuration for the database
type DatabaseConfig struct {
	Host string
	Port string
	User string
	Pass string
}

// ServerConfig holds the configuration for the server
type ServerConfig struct {
	Port string
}

// LoadConfig loads the configuration for the application
func LoadConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host: "localhost",
			Port: "3306",
			User: "root",
			Pass: "password",
		},
		Server: ServerConfig{
			Port: "8080",
		},
	}
}
