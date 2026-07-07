package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

type fileStorageEntry struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
}

type FileStorage struct {
	mu       sync.RWMutex
	data     map[string]urlEntry // id -> entry
	urlToID  map[string]string   // originalURL -> id
	userURLs map[string][]string // userID -> []id
	filePath string
}

func NewFileStorage(filePath string) (*FileStorage, error) {
	fs := &FileStorage{
		data:     make(map[string]urlEntry),
		urlToID:  make(map[string]string),
		userURLs: make(map[string][]string),
		filePath: filePath,
	}
	if err := fs.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return fs, nil
}

func (fs *FileStorage) Ping() error {
	return nil
}

// load читает данные из файла.
func (fs *FileStorage) load() error {
	file, err := os.Open(fs.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []fileStorageEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return err
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, entry := range entries {
		fs.data[entry.ShortURL] = urlEntry{
			originalURL: entry.OriginalURL,
			userID:      entry.UserID,
		}
		fs.urlToID[entry.OriginalURL] = entry.ShortURL
		if entry.UserID != "" {
			fs.userURLs[entry.UserID] = append(fs.userURLs[entry.UserID], entry.ShortURL)
		}
	}
	return nil
}

// save записывает данные в файл.
func (fs *FileStorage) save() error {
	entries := make([]fileStorageEntry, 0, len(fs.data))
	for id, entry := range fs.data {
		entries = append(entries, fileStorageEntry{
			UUID:        uuid.New().String(),
			ShortURL:    id,
			OriginalURL: entry.originalURL,
			UserID:      entry.userID,
		})
	}

	dir := filepath.Dir(fs.filePath)
	tmpFile, err := os.CreateTemp(dir, "storage.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(entries); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, fs.filePath)
}

// Save – обратная совместимость.
func (fs *FileStorage) Save(id, originalURL string) error {
	return fs.SaveForUser(id, originalURL, "")
}

// SaveForUser сохраняет с привязкой к пользователю.
func (fs *FileStorage) SaveForUser(id, originalURL, userID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if _, exists := fs.data[id]; exists {
		return ErrAlreadyExists
	}
	fs.data[id] = urlEntry{originalURL: originalURL, userID: userID}
	fs.urlToID[originalURL] = id
	if userID != "" {
		fs.userURLs[userID] = append(fs.userURLs[userID], id)
	}
	return fs.save()
}

// SaveBatch без привязки к пользователю.
func (fs *FileStorage) SaveBatch(urls map[string]string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := fs.data[id]; exists {
			return ErrAlreadyExists
		}
		fs.data[id] = urlEntry{originalURL: originalURL, userID: ""}
		fs.urlToID[originalURL] = id
	}
	return fs.save()
}

func (fs *FileStorage) FindByOriginal(originalURL string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if id, ok := fs.urlToID[originalURL]; ok {
		return id, nil
	}
	return "", ErrNotFound
}

func (fs *FileStorage) Load(id string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	entry, ok := fs.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return entry.originalURL, nil
}

// GetUserURLs возвращает все URL пользователя.
func (fs *FileStorage) GetUserURLs(userID string) ([]UserURL, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	ids, ok := fs.userURLs[userID]
	if !ok || len(ids) == 0 {
		return []UserURL{}, nil
	}
	result := make([]UserURL, 0, len(ids))
	for _, id := range ids {
		entry, exists := fs.data[id]
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

// SaveBatchForUser сохраняет несколько записей с привязкой к пользователю.
func (fs *FileStorage) SaveBatchForUser(urls map[string]string, userID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := fs.data[id]; exists {
			return ErrAlreadyExists
		}
		fs.data[id] = urlEntry{originalURL: originalURL, userID: userID}
		fs.urlToID[originalURL] = id
		if userID != "" {
			fs.userURLs[userID] = append(fs.userURLs[userID], id)
		}
	}
	return fs.save()
}
