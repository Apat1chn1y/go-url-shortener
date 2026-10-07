// Package main — точка входа в сервис сокращения URL.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/audit"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/config"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/grpcserver"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/server"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
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

// grpcBundle объединяет gRPC-сервер и listener, чтобы main оперировал одним значением.
type grpcBundle struct {
	server   *grpc.Server
	listener net.Listener
}

// setupGRPC создаёт gRPC-сервер, если задан cfg.GRPCAddress.
// Возвращает nil, если адрес пуст — это означает «gRPC выключен»,
// и main проверяет это одной строкой без обработки ошибок.
// Ошибки создания фатальны: приложение не должно стартовать с невалидным
// gRPC-сервером, поэтому здесь используется logger.Fatal.
func setupGRPC(cfg *config.Config, shortener *service.Shortener, logger zerolog.Logger) *grpcBundle {
	if cfg.GRPCAddress == "" {
		return nil
	}
	handler := grpcserver.NewShortenerServer(shortener, cfg.BaseURL, logger, cfg.AuthKey)
	srv, err := grpcserver.New(handler, cfg.EnableHTTPS, cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create gRPC server")
	}
	lis, err := grpcserver.Listen(cfg.GRPCAddress)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to listen on gRPC address")
	}
	return &grpcBundle{server: srv, listener: lis}
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
	router := handlers.NewRouter(handler, logger, cfg.AuthKey, cfg.TrustedSubnetNet)
	// Создание HTTP-сервера.
	srv := server.New(cfg.ServerAddress, router)

	// Инициализация gRPC-сервера (nil, если cfg.GRPCAddress не задан).
	grpcBundle := setupGRPC(cfg, shortener, logger)

	// Обрабатываем SIGINT, SIGTERM и SIGQUIT.
	// Контекст будет отменён при их получении
	shutdownCtx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // SIGINT
		syscall.SIGTERM, // SIGTERM
		syscall.SIGQUIT, // SIGQUIT
	)
	defer stop()

	// Запускаем HTTP-сервер в отдельной горутине.
	go func() {
		var runErr error
		if cfg.EnableHTTPS {
			logger.Info().
				Str("address", cfg.ServerAddress).
				Str("cert", cfg.TLSCertFile).
				Str("key", cfg.TLSKeyFile).
				Msg("Starting HTTPS server")
			runErr = srv.RunTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			logger.Info().Str("address", cfg.ServerAddress).Msg("Starting HTTP server")
			runErr = srv.Run()
		}
		// При штатном завершении Run/RunTLS возвращают http.ErrServerClosed — это не ошибка.
		// Любая другая ошибка (порт занят, сертификат не читается) означает падение сервера.
		if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
			logger.Error().Err(runErr).Msg("Server error")
			stop() // разблокируем main, чтобы он пошёл по обычному пути завершения
		}
	}()

	// Запускаем gRPC-сервер (если настроен) в отдельной горутине.
	if grpcBundle != nil {
		go func() {
			logger.Info().Str("address", cfg.GRPCAddress).Msg("Starting gRPC server")
			if err := grpcBundle.server.Serve(grpcBundle.listener); err != nil {
				logger.Error().Err(err).Msg("gRPC server error")
				stop()
			}
		}()
	}

	// Ожидаем сигнал завершения (или падение сервера)
	<-shutdownCtx.Done()
	logger.Info().Msg("Shutting down gracefully...")

	// Даём серверу 30 секунд, чтобы завершить все активные запросы.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Останавливаем HTTP-сервер (Shutdown дожидается завершения активных запросов).
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown error")
	}

	// Останавливаем gRPC-сервер.
	if grpcBundle != nil {
		grpcBundle.server.GracefulStop()
		logger.Info().Msg("gRPC server stopped")
	}

	// Останавливаем аудит: завершаем горутины, сбрасываем буферизованные события.
	auditManager.Close()
	logger.Info().Msg("Audit manager stopped")

	// Закрываем хранилище: сохраняем данные (для файлового) и освобождаем ресурсы (для PostgreSQL).
	if err := store.Close(); err != nil {
		logger.Error().Err(err).Msg("Storage close error")
	} else {
		logger.Info().Msg("Storage closed successfully")
	}

	logger.Info().Msg("Server stopped")
}
