// Package config предоставляет настройки сервера из командной строки, окружения и .env файла.
package config

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config хранит параметры запуска сервиса.
type Config struct {
	ServerAddress string // адрес и порт для запуска HTTP-сервера
	BaseURL       string // базовый URL для формирования коротких ссылок
}

// NewConfig загружает конфигурацию в следующем порядке:
// 1. Аргументы командной строки (-a, -b)
// 2. Переменные окружения (SERVER_ADDRESS, BASE_URL)
// 3. Файл .env
// 4. Значения по умолчанию
//
// Если BaseURL не задан явно, он формируется автоматически из ServerAddress
// (например, ":8080" -> "http://localhost:8080/", "localhost:43147" -> "http://localhost:43147/").
func NewConfig() *Config {
	// Загрузка .env файла (не критична, если файла нет)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env or defaults")
	}

	// Определяем флаги командной строки
	var serverAddr, baseURL string
	flag.StringVar(&serverAddr, "a", "", "адрес запуска HTTP-сервера (например, localhost:8888)")
	flag.StringVar(&baseURL, "b", "", "базовый адрес результирующего сокращённого URL (например, http://localhost:8888/)")
	flag.Parse()

	if serverAddr == "" {
		serverAddr = os.Getenv("SERVER_ADDRESS")
	}
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	if baseURL == "" {
		baseURL = os.Getenv("BASE_URL")
	}
	if baseURL == "" {
		baseURL = autoBaseURL(serverAddr)
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	return &Config{
		ServerAddress: serverAddr,
		BaseURL:       baseURL,
	}
}

// autoBaseURL автоматически создаёт базовый URL из адреса сервера.
// Примеры:
//
//	":8080" → "http://localhost:8080"
//	"localhost:43147" → "http://localhost:43147"
//	"127.0.0.1:8080" → "http://127.0.0.1:8080"
//	"0.0.0.0:8080" → "http://0.0.0.0:8080"
func autoBaseURL(addr string) string {
	host := strings.TrimPrefix(addr, "http://")
	host = strings.TrimPrefix(host, "https://")

	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}

	return "http://" + host
}
