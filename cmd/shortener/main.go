package main

import (
	"log"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/config"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handlers"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/server"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
)

func main() {

	// Загрузка конфигурации из переменных окружения.
	cfg := config.NewConfig()

	// Инициализация хранилища in-memory.
	store := storage.NewInMemoryStorage()

	// Инициализация сервиса бизнес-логики.
	shortener := service.NewShortener(store)
	// Инициализация HTTP-обработчика.
	handler := handlers.NewShortenHandler(shortener, cfg.BaseURL)

	// Создание роутера
	router := handlers.NewRouter(handler)

	// Создание и запуск HTTP-сервера.
	srv := server.New(cfg.ServerAddress, router)
	log.Printf("Starting server on %s", cfg.ServerAddress)
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
