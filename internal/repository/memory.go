package repository

import (
	"errors"
	"sync"
)

var (
	ErrNotFound = errors.New("URL not found")
)

// контракт для хранилища URL
type URLRepository interface {
	Save(id, originalURL string) error
	Find(id string) (string, error)
}

// репозиторий для хранения соответствия в map с мьютексом
type InMemoryRepository struct {
	mu    sync.RWMutex
	store map[string]string // id -> originalURL
}

// создаёт новый экземпляр in-memory репозитория
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		store: make(map[string]string),
	}
}

// сохраняет пару id -> URL
func (r *InMemoryRepository) Save(id, originalURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[id] = originalURL
	return nil
}

// возвращает оригинальный URL по id
func (r *InMemoryRepository) Find(id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	original, ok := r.store[id]
	if !ok {
		return "", ErrNotFound
	}
	return original, nil
}
