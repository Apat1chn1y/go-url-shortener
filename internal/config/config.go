// Package config предоставляет настройки сервера.
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config хранит параметры запуска сервиса.
type Config struct {
	ServerAddress string // адрес и порт для запуска HTTP-сервера
	BaseURL       string // базовый URL для формирования коротких ссылок
}

// NewConfig загружает конфигурацию из .env и переменных окружения.
// Если .env отсутствует, используются системные переменные.
// Значения по умолчанию: SERVER_ADDRESS=":8080", BASE_URL="http://localhost:8080/".
func NewConfig() *Config {
	// Загрузка .env файла (не критична, если файла нет)
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system env")
	}

	serverAddr := os.Getenv("SERVER_ADDRESS")
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080/"
	}

	return &Config{
		ServerAddress: serverAddr,
		BaseURL:       baseURL,
	}
}
