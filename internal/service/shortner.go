// Package service предоставляет бизнес-логику сокращения URL.
package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
)

// idLength — длина генерируемого короткого идентификатора.
const idLength = 8

// Shortener реализует бизнес-логику сокращения URL.
type Shortener struct {
	storage storage.Storage
}

// NewShortener создаёт новый сервис сокращения URL с указанным хранилищем.
func NewShortener(storage storage.Storage) *Shortener {
	return &Shortener{storage: storage}
}

// generateID генерирует случайный строковый идентификатор длины idLength.
func generateID() (string, error) {
	buf := make([]byte, idLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := base64.URLEncoding.EncodeToString(buf)
	id = strings.TrimRight(id, "=")
	if len(id) > idLength {
		id = id[:idLength]
	}
	return id, nil
}

// Create создаёт короткий URL для переданного оригинального.
// Возвращает полный короткий URL (baseURL + id) или ошибку.
func (s *Shortener) Create(originalURL, baseURL string) (string, error) {
	if originalURL == "" {
		return "", errors.New("empty URL")
	}
	for attempts := 0; attempts < 10; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		err = s.storage.Save(id, originalURL)
		if err == nil {
			return baseURL + id, nil
		}
		if !errors.Is(err, storage.ErrAlreadyExists) {
			return "", err
		}
	}
	return "", errors.New("failed to generate unique ID")
}

// Get возвращает оригинальный URL по короткому идентификатору.
// Возвращает ошибку, если идентификатор пуст или не найден.
func (s *Shortener) Get(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	return s.storage.Load(id)
}
