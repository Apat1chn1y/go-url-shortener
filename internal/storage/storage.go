// Package storage предоставляет интерфейс для работы с хранилищем URL.
package storage

import "errors"

// ErrNotFound возвращается, если запрошенный URL не найден в хранилище.
var ErrNotFound = errors.New("URL not found")

// ErrAlreadyExists возвращается при попытке сохранить уже существующий ID.
var ErrAlreadyExists = errors.New("ID already exists")

// ErrOriginalURLDuplicate возвращается при попытке сохранить уже существующий URL.
var ErrOriginalURLDuplicate = errors.New("original URL already exists")

// Storage определяет контракт для хранилища коротких URL.
type Storage interface {
	// Save сохраняет пару идентификатор-оригинальный URL.
	// Возвращает ошибку, если идентификатор уже существует.
	Save(id, originalURL string) error

	// Load возвращает оригинальный URL по идентификатору.
	// Возвращает ErrNotFound, если идентификатор отсутствует.
	Load(id string) (string, error)

	Ping() error

	SaveBatch(urls map[string]string) error

	FindByOriginal(originalURL string) (string, error)
}
