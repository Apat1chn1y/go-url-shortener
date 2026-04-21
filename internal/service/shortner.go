package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/repository"
)

const idLength = 8 // длина короткого идентификатора

// бизнес-логика сокращения URL
type ShortenerService struct {
	repo repository.URLRepository
}

// создаёт новый сервис с указанным репозиторием
func NewShortenerService(repo repository.URLRepository) *ShortenerService {
	return &ShortenerService{repo: repo}
}

// генерирует случайный строковый идентификатор заданной длины
func generateID() (string, error) {
	bytes := make([]byte, idLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	id := base64.URLEncoding.EncodeToString(bytes)
	id = strings.TrimRight(id, "=")
	if len(id) > idLength {
		id = id[:idLength]
	}
	return id, nil
}

// создаёт короткий URL для переданного оригинального
func (s *ShortenerService) CreateShortURL(originalURL string, baseURL string) (string, error) {
	if originalURL == "" {
		return "", errors.New("empty URL")
	}

	for attempts := 0; attempts < 10; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}

		if _, err := s.repo.Find(id); err == repository.ErrNotFound {

			if err := s.repo.Save(id, originalURL); err != nil {
				return "", err
			}
			return baseURL + id, nil
		}
	}
	return "", errors.New("failed to generate unique ID")
}

// возвращает оригинальный URL по короткому идентификатору
func (s *ShortenerService) GetOriginalURL(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	return s.repo.Find(id)
}
