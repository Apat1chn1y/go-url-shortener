package main

import (
	"os"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/config"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/server"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
)

func main() {
	// Настройка логгера: вывод в stdout в формате JSON (без ConsoleWriter, чтобы избежать паники)
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().Timestamp().Logger()

	// Загрузка конфигурации из переменных окружения.
	cfg := config.NewConfig()
	// Инициализация хранилища in-memory.
	// store := storage.NewInMemoryStorage()
	// Инициализация файлового хранилища.
	store, err := storage.NewFileStorage(cfg.FileStoragePath)
	if err != nil {
		logger.Fatal().Err(err).Str("path", cfg.FileStoragePath).Msg("Cannot initialize file storage")
	}
	// Инициализация сервиса бизнес-логики.
	shortener := service.NewShortener(store)
	// Инициализация HTTP-обработчика.
	handler := handlers.NewShortenHandler(shortener, cfg.BaseURL)

	// Создание роутера
	router := handlers.NewRouter(handler, logger)
	// Создание и запуск HTTP-сервера.
	srv := server.New(cfg.ServerAddress, router)

	logger.Info().Str("address", cfg.ServerAddress).Msg("Starting server")
	if err := srv.Run(); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}
