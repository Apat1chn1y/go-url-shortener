// Package config предоставляет настройки сервера из переменных окружения, аргументов командной строки и .env файла.
// Приоритет: переменная окружения > флаг командной строки > значение по умолчанию.
package config

import (
	"flag"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config хранит параметры запуска сервиса.
type Config struct {
	ServerAddress   string // адрес и порт для запуска HTTP-сервера
	BaseURL         string // базовый URL для формирования коротких ссылок
	FileStoragePath string // путь к файлу для хранения данных
}

// NewConfig загружает конфигурацию в следующем порядке приоритета:
//  1. Переменные окружения (SERVER_ADDRESS, BASE_URL)
//  2. Аргументы командной строки (-a, -b)
//  3. Значения по умолчанию
//  4. Если BaseURL всё ещё не задан, он автоматически формируется из ServerAddress.
//
// Примеры:
//
//	export SERVER_ADDRESS=:9090 -> ServerAddress=":9090"
//	go run . -a :8888            -> ServerAddress=":8888" (если нет SERVER_ADDRESS)
//	без параметров               -> ServerAddress=":8080", BaseURL="http://localhost:8080/"
func NewConfig() *Config {
	// Загрузка .env (если файл существует) – значения не перезаписывают уже установленные переменные окружения
	_ = godotenv.Load() // игнорируем ошибку отсутствия файла

	// Определяем флаги командной строки
	var flagServerAddr, flagBaseURL, flagFile string
	flag.StringVar(&flagServerAddr, "a", "", "адрес запуска HTTP-сервера (например, localhost:8888)")
	flag.StringVar(&flagBaseURL, "b", "", "базовый адрес результирующего сокращённого URL (например, http://localhost:8888/)")
	flag.StringVar(&flagFile, "f", "", "путь к файлу хранения данных")
	flag.Parse()

	serverAddr := os.Getenv("SERVER_ADDRESS")
	if serverAddr == "" {
		serverAddr = flagServerAddr
	}
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = flagBaseURL
	}
	if baseURL == "" {
		baseURL = autoBaseURL(serverAddr)
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	filePath := os.Getenv("FILE_STORAGE_PATH")
	if filePath == "" {
		filePath = flagFile
	}
	if filePath == "" {
		filePath = "storage.json" // значение по умолчанию
	}

	return &Config{
		ServerAddress:   serverAddr,
		BaseURL:         baseURL,
		FileStoragePath: filePath,
	}
}

// autoBaseURL преобразует адрес сервера в HTTP-URL.
// Примеры:
//
//	":8080"      → "http://localhost:8080"
//	"localhost:43147" → "http://localhost:43147"
//	"127.0.0.1:9090"  → "http://127.0.0.1:9090"
func autoBaseURL(addr string) string {
	host := strings.TrimPrefix(addr, "http://")
	host = strings.TrimPrefix(host, "https://")
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	return "http://" + host
}
