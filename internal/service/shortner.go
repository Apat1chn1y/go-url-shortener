// Package service предоставляет бизнес-логику сокращения URL.
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

// Ping проверяет доступность хранилища.
func (s *Shortener) Ping() error {
	return s.storage.Ping()
}

// BatchItem представляет входной элемент батча.
type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchResult представляет выходной элемент батча.
type BatchResult struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// CreateBatch создаёт короткие URL для множества оригинальных.
// Возвращает срез результатов с correlation_id и полными short_url.
func (s *Shortener) CreateBatch(items []BatchItem, baseURL string) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, errors.New("empty batch")
	}

	// Проверяем, что все original_url не пустые
	for _, item := range items {
		if item.OriginalURL == "" {
			return nil, ErrEmptyOriginalURL // объявлена в пакете service
		}
	}

	results := make([]BatchResult, 0, len(items))
	var toSave = make(map[string]string)         // id -> originalURL
	var correlationMap = make(map[string]string) // id -> correlationID

	for _, item := range items {
		// Проверяем существование
		existingID, err := s.storage.FindByOriginal(item.OriginalURL)
		if err == nil {
			// Уже существует – добавляем в результат без сохранения
			results = append(results, BatchResult{
				CorrelationID: item.CorrelationID,
				ShortURL:      baseURL + existingID,
			})
			continue
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("find original: %w", err)
		}
		// Генерируем ID
		var id string
		for attempts := 0; attempts < 10; attempts++ {
			id, err = generateID()
			if err != nil {
				return nil, err
			}
			// Проверяем, не занят ли ID (можно положиться на SaveBatch)
			break
		}
		toSave[id] = item.OriginalURL
		correlationMap[id] = item.CorrelationID
	}

	if len(toSave) > 0 {
		if err := s.storage.SaveBatch(toSave); err != nil {
			return nil, err
		}
		// Добавляем новые записи в результаты
		for id, originalURL := range toSave {
			_ = originalURL
			results = append(results, BatchResult{
				CorrelationID: correlationMap[id],
				ShortURL:      baseURL + id,
			})
		}
	}
	return results, nil
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
		return "", ErrEmptyURL
	}

	// Проверяем, существует ли уже такой URL
	existingID, err := s.storage.FindByOriginal(originalURL)
	if err == nil {
		return baseURL + existingID, ErrURLAlreadyExists
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("find original: %w", err)
	}

	// Генерируем новый ID и сохраняем
	for attempts := 0; attempts < 10; attempts++ {
		id, err := generateID()
		if err != nil {
			return "", err
		}
		err = s.storage.Save(id, originalURL)
		if err == nil {
			return baseURL + id, nil
		}
		if errors.Is(err, storage.ErrAlreadyExists) {
			continue // коллизия ID
		}
		return "", err
	}
	return "", ErrMaxAttemptsExceeded
}

// Get возвращает оригинальный URL по короткому идентификатору.
// Возвращает ошибку, если идентификатор пуст или не найден.
func (s *Shortener) Get(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	return s.storage.Load(id)
}
