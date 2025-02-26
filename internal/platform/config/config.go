package config

import "os"

// Config tiene la configuración para la aplicación
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
}

// DatabaseConfig tiene la configuración para la base de datos
type DatabaseConfig struct {
	Host string
	Port string
	User string
	Pass string
}

// ServerConfig tiene la configuración para el servidor
type ServerConfig struct {
	Port string
}

// LoadConfig carga la configuración de la aplicación desde las variables de entorno
func LoadConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host: os.Getenv("DB_HOST"),
			Port: os.Getenv("DB_PORT"),
			User: os.Getenv("DB_USER"),
			Pass: os.Getenv("DB_PASS"),
		},
		Server: ServerConfig{
			Port: os.Getenv("SERVER_PORT"),
		},
	}
}
