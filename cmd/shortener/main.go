// Package main — точка входа в сервис сокращения URL.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/config"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/server"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
)

// Информация о сборке, заполняется при линковке через -ldflags.
var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

// printBuildInfo выводит информацию о версии, дате и коммите сборки.
func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

func main() {
	// Выводим информацию о сборке
	printBuildInfo()

	// Настройка логгера: вывод в stdout в формате JSON (без ConsoleWriter, чтобы избежать паники)
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().Timestamp().Logger()

	var store storage.Storage
	var cfg *config.Config
	var err error

	// Загрузка конфигурации из переменных окружения.
	cfg, err = config.NewConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Приоритет хранилищ: PostgreSQL → файл → память
	if cfg.DatabaseDSN != "" {
		// Инициализация pg хранилища.
		store, err = storage.NewPostgresStorage(cfg.DatabaseDSN)
		if err != nil {
			logger.Fatal().Err(err).Msg("Cannot connect to database")
		}
		defer store.(*storage.PostgresStorage).Close()
		logger.Info().Msg("Using PostgreSQL storage")
	} else if cfg.FileStoragePath != "" {
		// Инициализация файлового хранилища.
		store, err = storage.NewFileStorage(cfg.FileStoragePath)
		if err != nil {
			logger.Fatal().Err(err).Str("path", cfg.FileStoragePath).Msg("Cannot initialize file storage")
		}
		logger.Info().Msg("Using file storage")
	} else {
		// Инициализация хранилища in-memory.
		store = storage.NewInMemoryStorage()
		logger.Info().Msg("Using in-memory storage")
	}

	// Инициализация аудита
	auditManager := audit.NewManager()
	if cfg.AuditFilePath != "" {
		fileWriter, err := audit.NewFileWriter(cfg.AuditFilePath)
		if err != nil {
			logger.Error().Err(err).Str("path", cfg.AuditFilePath).Msg("Failed to create audit file writer")
		} else {
			defer fileWriter.Close() // закрываем файл при завершении программы
			auditManager.AddWriter(fileWriter)
			logger.Info().Str("path", cfg.AuditFilePath).Msg("Audit file writer enabled")
		}
	}
	if cfg.AuditURL != "" {
		httpWriter := audit.NewHTTPWriter(cfg.AuditURL)
		auditManager.AddWriter(httpWriter)
		logger.Info().Str("url", cfg.AuditURL).Msg("Audit HTTP writer enabled")
	}

	// Инициализация сервиса бизнес-логики.
	shortener := service.NewShortener(store)
	// Инициализация HTTP-обработчика.
	handler := handlers.NewShortenHandler(shortener, cfg.BaseURL, logger, auditManager)

	// Создание роутера
	router := handlers.NewRouter(handler, logger, cfg.AuthKey)
	// Создание и запуск HTTP-сервера.
	srv := server.New(cfg.ServerAddress, router)
	logger.Info().Str("address", cfg.ServerAddress).Msg("Starting server")

	// Используем signal.NotifyContext для graceful shutdown.
	// Контекст будет отменён при получении SIGINT или SIGTERM.
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.Run(); err != nil {
			logger.Error().Err(err).Msg("Server error")
		}
	}()

	// Ожидаем сигнал завершения
	<-shutdownCtx.Done()
	logger.Info().Msg("Shutting down gracefully...")

	// Контекст с таймаутом для завершения операций
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Останавливаем HTTP-сервер
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	// Закрываем аудит (останавливает горутины, закрывает каналы)
	auditManager.Close()
	logger.Info().Msg("Server stopped")
}
