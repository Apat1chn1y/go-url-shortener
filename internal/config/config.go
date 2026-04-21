package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func NewConfig() *Config {

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
