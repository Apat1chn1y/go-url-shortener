package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
)

var (
	ErrEmptyURL            = errors.New("empty URL")
	ErrMaxAttemptsExceeded = errors.New("failed to generate unique ID after 10 attempts")
	ErrEmptyOriginalURL    = errors.New("empty original_url in batch")
	ErrURLAlreadyExists    = errors.New("URL already exists")
)

const idLength = 8

type Shortener struct {
	storage storage.Storage
}

func NewShortener(s storage.Storage) *Shortener {
	return &Shortener{storage: s}
}

func (s *Shortener) Ping() error {
	return s.storage.Ping()
}

func (s *Shortener) FindByOriginal(originalURL string) (string, error) {
	return s.storage.FindByOriginal(originalURL)
}

// Create теперь принимает userID.
func (s *Shortener) Create(originalURL, baseURL, userID string) (string, error) {
	if originalURL == "" {
		return "", ErrEmptyURL
	}
	existingID, err := s.storage.FindByOriginal(originalURL)
	if err == nil {
		return baseURL + existingID, ErrURLAlreadyExists
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("find original: %w", err)
	}
	for attempts := 0; attempts < 10; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		err = s.storage.SaveForUser(id, originalURL, userID)
		if err == nil {
			return baseURL + id, nil
		}
		if errors.Is(err, storage.ErrAlreadyExists) {
			continue
		}
		return "", err
	}
	return "", ErrMaxAttemptsExceeded
}

func (s *Shortener) Get(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	return s.storage.Load(id)
}

type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResult struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// CreateBatch теперь принимает userID и сохраняет с привязкой, если URL новый.
// Для существующих URL возвращает существующий ID без изменения владельца.
func (s *Shortener) CreateBatch(items []BatchItem, baseURL, userID string) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}
	results := make([]BatchResult, len(items))
	type pendingItem struct {
		correlationID string
		originalURL   string
		id            string
	}
	var pending []pendingItem

	for i, item := range items {
		if item.OriginalURL == "" {
			return nil, ErrEmptyOriginalURL
		}
		existingID, err := s.storage.FindByOriginal(item.OriginalURL)
		if err == nil {
			results[i] = BatchResult{
				CorrelationID: item.CorrelationID,
				ShortURL:      baseURL + existingID,
			}
			continue
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("find original: %w", err)
		}
		var id string
		for attempts := 0; attempts < 10; attempts++ {
			id, err = generateID()
			if err != nil {
				return nil, err
			}
			break
		}
		pending = append(pending, pendingItem{
			correlationID: item.CorrelationID,
			originalURL:   item.OriginalURL,
			id:            id,
		})
		results[i] = BatchResult{
			CorrelationID: item.CorrelationID,
			ShortURL:      baseURL + id,
		}
	}

	if len(pending) > 0 {
		toSave := make(map[string]string, len(pending))
		for _, p := range pending {
			toSave[p.id] = p.originalURL
		}
		if err := s.storage.SaveBatchForUser(toSave, userID); err != nil {
			return nil, fmt.Errorf("save batch for user: %w", err)
		}
	}

	return results, nil
}

// GetUserURLs возвращает URL пользователя.
func (s *Shortener) GetUserURLs(userID string) ([]storage.UserURL, error) {
	return s.storage.GetUserURLs(userID)
}

// generateID генерирует случайный ID.
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
