// Package storage предоставляет интерфейс и реализации хранилища URL.
package storage

import (
	"sync"
)

// InMemoryStorage реализует Storage с использованием map и мьютекса.
type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string]string // id -> originalURL
}

// NewInMemoryStorage создаёт новый экземпляр in-memory хранилища.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data: make(map[string]string),
	}
}

func (s *InMemoryStorage) Ping() error {
	return nil
}

// Save сохраняет пару id -> originalURL.
// Возвращает ErrAlreadyExists, если id уже занят.
func (s *InMemoryStorage) Save(id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; exists {
		return ErrAlreadyExists
	}
	s.data[id] = originalURL
	return nil
}

// Load возвращает оригинальный URL по id.
// Возвращает ErrNotFound, если id не найден.
func (s *InMemoryStorage) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	original, ok := s.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return original, nil
}
