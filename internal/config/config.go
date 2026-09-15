// Package config предоставляет настройки сервера из переменных окружения, аргументов командной строки и .env файла.
// Приоритет: переменная окружения > флаг командной строки > значение по умолчанию.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config содержит все параметры конфигурации приложения.
type Config struct {
	ServerAddress   string
	BaseURL         string
	FileStoragePath string
	DatabaseDSN     string // строка подключения к базе данных
	AuthKey         []byte
	AuditFilePath   string // путь к файлу аудита (если не пуст)
	AuditURL        string // URL удаленного сервера аудита (если не пуст)
	EnableHTTPS     bool   // включает HTTPS, если true
	TLSCertFile     string // путь к файлу сертификата
	TLSKeyFile      string // путь к файлу приватного ключа
}

// generateRandomKey создаёт случайный 32-байтовый ключ в base64.
// Возвращает ошибку, если не удалось прочитать случайные данные.
func generateRandomKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random auth key: %w", err)
	}
	key := make([]byte, base64.URLEncoding.EncodedLen(len(b)))
	base64.URLEncoding.Encode(key, b)
	return key, nil
}

// NewConfig загружает конфигурацию и возвращает ошибку, если не удалось сгенерировать ключ.
func NewConfig() (*Config, error) {
	// Загрузка .env (если файл существует)
	_ = godotenv.Load()

	// Определяем флаги командной строки
	var flagAddr, flagBase, flagFile, flagDB, flagAuditFile, flagAuditURL string
	var flagCert, flagKey string
	var flagEnableHTTPS bool
	flag.StringVar(&flagAddr, "a", "", "адрес сервера")
	flag.StringVar(&flagBase, "b", "", "базовый URL")
	flag.StringVar(&flagFile, "f", "", "путь к файлу хранения данных")
	flag.StringVar(&flagDB, "d", "", "DSN для подключения к PostgreSQL")
	flag.StringVar(&flagAuditFile, "audit-file", "", "путь к файлу аудита")
	flag.StringVar(&flagAuditURL, "audit-url", "", "URL удаленного сервера аудита")
	flag.BoolVar(&flagEnableHTTPS, "s", false, "включить HTTPS")
	flag.StringVar(&flagCert, "cert", "cert.pem", "путь к файлу TLS-сертификата")
	flag.StringVar(&flagKey, "key", "key.pem", "путь к файлу приватного TLS-ключа")
	flag.Parse()

	addr, ok := os.LookupEnv("SERVER_ADDRESS")
	if !ok {
		addr = flagAddr
	}
	if addr == "" {
		addr = ":8080"
	}

	base, ok := os.LookupEnv("BASE_URL")
	if !ok {
		base = flagBase
	}
	if base == "" {
		base = autoBaseURL(addr)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	filePath, ok := os.LookupEnv("FILE_STORAGE_PATH")
	if !ok {
		filePath = flagFile
	}

	dbDSN, ok := os.LookupEnv("DATABASE_DSN")
	if !ok {
		dbDSN = flagDB
	}

	auditFile, ok := os.LookupEnv("AUDIT_FILE")
	if !ok {
		auditFile = flagAuditFile
	}

	auditURL, ok := os.LookupEnv("AUDIT_URL")
	if !ok {
		auditURL = flagAuditURL
	}

	// ENABLE_HTTPS: переменная окружения имеет приоритет над флагом
	enableHTTPS := flagEnableHTTPS
	if v, ok := os.LookupEnv("ENABLE_HTTPS"); ok {
		enableHTTPS = v == "true" || v == "1"
	}

	certFile := flagCert
	if v, ok := os.LookupEnv("TLS_CERT_FILE"); ok {
		certFile = v
	}

	keyFile := flagKey
	if v, ok := os.LookupEnv("TLS_KEY_FILE"); ok {
		keyFile = v
	}

	var authKey []byte
	var err error
	keyStr, ok := os.LookupEnv("AUTH_KEY")
	if !ok {
		authKey, err = generateRandomKey()
		if err != nil {
			return nil, fmt.Errorf("generate auth key: %w", err)
		}
	} else {
		authKey = []byte(keyStr)
	}

	return &Config{
		ServerAddress:   addr,
		BaseURL:         base,
		FileStoragePath: filePath,
		DatabaseDSN:     dbDSN,
		AuthKey:         authKey,
		AuditFilePath:   auditFile,
		AuditURL:        auditURL,
		EnableHTTPS:     enableHTTPS,
		TLSCertFile:     certFile,
		TLSKeyFile:      keyFile,
	}, nil
}

// autoBaseURL преобразует адрес сервера в HTTP-URL.
func autoBaseURL(addr string) string {
	host := strings.TrimPrefix(addr, "http://")
	host = strings.TrimPrefix(host, "https://")
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	return "http://" + host
}
