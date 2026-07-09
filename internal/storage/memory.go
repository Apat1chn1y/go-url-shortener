package storage

import (
	"sync"
)

// InMemoryStorage реализует Storage с использованием map и мьютекса.
type InMemoryStorage struct {
	mu       sync.RWMutex
	data     map[string]urlEntry // id -> entry
	urlToID  map[string]string   // originalURL -> id
	userURLs map[string][]string // userID -> []id
}

// NewInMemoryStorage создаёт новый экземпляр in-memory хранилища.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		data:     make(map[string]urlEntry),
		urlToID:  make(map[string]string),
		userURLs: make(map[string][]string),
	}
}

// Save – обратная совместимость, вызывает SaveForUser с пустым userID.
func (s *InMemoryStorage) Save(id, originalURL string) error {
	return s.SaveForUser(id, originalURL, "")
}

// SaveForUser сохраняет пару id->originalURL с привязкой к пользователю.
func (s *InMemoryStorage) SaveForUser(id, originalURL, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[id]; exists {
		return ErrAlreadyExists
	}
	s.data[id] = urlEntry{originalURL: originalURL, userID: userID, deleted: false}
	s.urlToID[originalURL] = id
	if userID != "" {
		s.userURLs[userID] = append(s.userURLs[userID], id)
	}
	return nil
}

// SaveBatch – массовое сохранение без userID.
func (s *InMemoryStorage) SaveBatch(urls map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := s.data[id]; exists {
			return ErrAlreadyExists
		}
		s.data[id] = urlEntry{originalURL: originalURL, userID: "", deleted: false}
		s.urlToID[originalURL] = id
	}
	return nil
}

// SaveBatchForUser – массовое сохранение с привязкой к пользователю.
func (s *InMemoryStorage) SaveBatchForUser(urls map[string]string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := s.data[id]; exists {
			return ErrAlreadyExists
		}
		s.data[id] = urlEntry{originalURL: originalURL, userID: userID, deleted: false}
		s.urlToID[originalURL] = id
		if userID != "" {
			s.userURLs[userID] = append(s.userURLs[userID], id)
		}
	}
	return nil
}

// Load возвращает оригинальный URL по id, если запись не удалена.
// Если запись удалена, возвращает ErrGone.
func (s *InMemoryStorage) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.data[id]
	if !ok {
		return "", ErrNotFound
	}
	if entry.deleted {
		return "", ErrGone
	}
	return entry.originalURL, nil
}

// FindByOriginal возвращает id по оригинальному URL, игнорируя удалённые записи.
func (s *InMemoryStorage) FindByOriginal(originalURL string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.urlToID[originalURL]; ok {
		entry := s.data[id]
		if !entry.deleted {
			return id, nil
		}
	}
	return "", ErrNotFound
}

// GetUserURLs возвращает все НЕ УДАЛЁННЫЕ URL пользователя.
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
		if !exists || entry.deleted {
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

// DeleteUserURLs помечает URL как удалённые для данного пользователя.
func (s *InMemoryStorage) DeleteUserURLs(userID string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if entry, exists := s.data[id]; exists && entry.userID == userID {
			entry.deleted = true
			s.data[id] = entry
			// Удаляем из списка пользователя
			if userID != "" {
				list := s.userURLs[userID]
				for i, v := range list {
					if v == id {
						s.userURLs[userID] = append(list[:i], list[i+1:]...)
						break
					}
				}
			}
		}
	}
	return nil
}

func (s *InMemoryStorage) Ping() error {
	return nil
}
