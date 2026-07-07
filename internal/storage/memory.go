package storage

import (
	"sync"
)

type urlEntry struct {
	originalURL string
	userID      string
}

type InMemoryStorage struct {
	mu       sync.RWMutex
	data     map[string]urlEntry // id -> entry
	urlToID  map[string]string   // originalURL -> id
	userURLs map[string][]string // userID -> []id
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data:     make(map[string]urlEntry),
		urlToID:  make(map[string]string),
		userURLs: make(map[string][]string),
	}
}

func (s *InMemoryStorage) Ping() error {
	return nil
}

// Save – обратная совместимость.
func (s *InMemoryStorage) Save(id, originalURL string) error {
	return s.SaveForUser(id, originalURL, "")
}

// SaveForUser сохраняет пару с привязкой к пользователю.
func (s *InMemoryStorage) SaveForUser(id, originalURL, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; exists {
		return ErrAlreadyExists
	}
	s.data[id] = urlEntry{originalURL: originalURL, userID: userID}
	s.urlToID[originalURL] = id
	if userID != "" {
		s.userURLs[userID] = append(s.userURLs[userID], id)
	}
	return nil
}

// SaveBatch – без привязки к пользователю.
func (s *InMemoryStorage) SaveBatch(urls map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := s.data[id]; exists {
			return ErrAlreadyExists
		}
		s.data[id] = urlEntry{originalURL: originalURL, userID: ""}
		s.urlToID[originalURL] = id
	}
	return nil
}

func (s *InMemoryStorage) FindByOriginal(originalURL string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.urlToID[originalURL]; ok {
		return id, nil
	}
	return "", ErrNotFound
}

func (s *InMemoryStorage) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return entry.originalURL, nil
}

// GetUserURLs возвращает все URL пользователя.
func (s *InMemoryStorage) GetUserURLs(userID string) ([]UserURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids, ok := s.userURLs[userID]
	if !ok || len(ids) == 0 {
		return []UserURL{}, nil
	}
	result := make([]UserURL, 0, len(ids))
	for _, id := range ids {
		entry, exists := s.data[id]
		if !exists {
			continue
		}
		result = append(result, UserURL{
			ID:          id,
			ShortURL:    "", // заполняется в хендлере
			OriginalURL: entry.originalURL,
		})
	}
	return result, nil
}

// SaveBatchForUser сохраняет несколько записей с привязкой к одному пользователю.
func (s *InMemoryStorage) SaveBatchForUser(urls map[string]string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := s.data[id]; exists {
			return ErrAlreadyExists
		}
		s.data[id] = urlEntry{originalURL: originalURL, userID: userID}
		s.urlToID[originalURL] = id
		if userID != "" {
			s.userURLs[userID] = append(s.userURLs[userID], id)
		}
	}
	return nil
}
