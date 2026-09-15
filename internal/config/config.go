// Package config предоставляет настройки сервера из переменных окружения, аргументов командной строки,
// .env файла и JSON-файла конфигурации.
// Приоритет: флаг командной строки > переменная окружения > JSON-файл > значение по умолчанию.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

// fileConfig описывает структуру JSON-файла конфигурации.
type fileConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	EnableHTTPS     bool   `json:"enable_https"`
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

// stripComments удаляет однострочные комментарии // из JSON-данных,
// корректно обрабатывая строковые значения (комментарии внутри строк сохраняются).
func stripComments(data []byte) []byte {
	result := make([]byte, 0, len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			result = append(result, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			result = append(result, c)
			continue
		}
		if c == '/' && i+1 < len(data) && data[i+1] == '/' {
			// пропускаем до конца строки (не включая '\n')
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				result = append(result, data[i])
			}
			continue
		}
		result = append(result, c)
	}
	return result
}

// loadFileConfig читает JSON-файл конфигурации. Комментарии // удаляются перед парсингом.
func loadFileConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	cleaned := stripComments(data)

	var fc fileConfig
	if err := json.Unmarshal(cleaned, &fc); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &fc, nil
}

// NewConfig загружает конфигурацию и возвращает ошибку, если не удалось сгенерировать ключ.
//
// Приоритет значений (от старшего к младшему):
//  1. Флаг командной строки
//  2. Переменная окружения
//  3. JSON-файл конфигурации (флаг -c/-config или CONFIG)
//  4. Значение по умолчанию
func NewConfig() (*Config, error) {
	// Загрузка .env (если файл существует)
	_ = godotenv.Load()

	// Определяем флаги командной строки
	var flagAddr, flagBase, flagFile, flagDB, flagAuditFile, flagAuditURL string
	var flagCert, flagKey, flagConfigPath string
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
	flag.StringVar(&flagConfigPath, "c", "", "путь к JSON-файлу конфигурации")
	flag.StringVar(&flagConfigPath, "config", "", "путь к JSON-файлу конфигурации")
	flag.Parse()

	// Определяем, какие флаги были явно переданы
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// Определяем путь к JSON-файлу конфигурации: флаг -c/-config > CONFIG
	configPath := ""
	if setFlags["c"] {
		configPath = flagConfigPath
	} else if setFlags["config"] {
		configPath = flagConfigPath
	} else if v, ok := os.LookupEnv("CONFIG"); ok && v != "" {
		configPath = v
	}

	// Загружаем JSON-файл, если путь задан
	var fc fileConfig
	if configPath != "" {
		loaded, err := loadFileConfig(configPath)
		if err != nil {
			return nil, err
		}
		fc = *loaded
	}

	// Вспомогательные функции для определения итогового значения
	// по приоритету: flag > env > file > default.
	pickString := func(flagName, flagValue, envName, fileValue, defaultVal string) string {
		if setFlags[flagName] && flagValue != "" {
			return flagValue
		}
		if v, ok := os.LookupEnv(envName); ok && v != "" {
			return v
		}
		if fileValue != "" {
			return fileValue
		}
		return defaultVal
	}
	pickBool := func(flagName string, flagValue bool, envName string, fileValue bool) bool {
		if setFlags[flagName] {
			return flagValue
		}
		if v, ok := os.LookupEnv(envName); ok {
			return v == "true" || v == "1"
		}
		return fileValue
	}

	// SERVER_ADDRESS
	addr := pickString("a", flagAddr, "SERVER_ADDRESS", fc.ServerAddress, ":8080")

	// BASE_URL
	base := pickString("b", flagBase, "BASE_URL", fc.BaseURL, "")
	if base == "" {
		base = autoBaseURL(addr)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	// FILE_STORAGE_PATH
	filePath := pickString("f", flagFile, "FILE_STORAGE_PATH", fc.FileStoragePath, "")

	// DATABASE_DSN
	dbDSN := pickString("d", flagDB, "DATABASE_DSN", fc.DatabaseDSN, "")

	// AUDIT_FILE
	auditFile := pickString("audit-file", flagAuditFile, "AUDIT_FILE", "", "")

	// AUDIT_URL
	auditURL := pickString("audit-url", flagAuditURL, "AUDIT_URL", "", "")

	// ENABLE_HTTPS
	enableHTTPS := pickBool("s", flagEnableHTTPS, "ENABLE_HTTPS", fc.EnableHTTPS)

	// TLS_CERT_FILE
	certFile := pickString("cert", flagCert, "TLS_CERT_FILE", "", "cert.pem")

	// TLS_KEY_FILE
	keyFile := pickString("key", flagKey, "TLS_KEY_FILE", "", "key.pem")

	// AUTH_KEY
	var authKey []byte
	var err error
	keyStr, ok := os.LookupEnv("AUTH_KEY")
	if !ok || keyStr == "" {
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
// Примеры:
//
//	":8080"           → "http://localhost:8080"
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
