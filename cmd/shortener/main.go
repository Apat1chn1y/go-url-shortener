package main

import (
	"log"
	"net/http"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/config"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/handler"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/repository"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
)

func main() {
	// инициализация конфигурации
	cfg := config.NewConfig()

	// инициализация репозитория (хранения в памяти)
	repo := repository.NewInMemoryRepository()

	// инициализация сервиса
	shortenerService := service.NewShortenerService(repo)

	// инициализация HTTP-обработчика
	h := handler.NewHandler(shortenerService, cfg.BaseURL)

	// запуск сервера
	log.Printf("Starting server on %s", cfg.ServerAddress)
	if err := http.ListenAndServe(cfg.ServerAddress, h); err != nil {
		log.Fatal("Server failed: ", err)
	}
}
