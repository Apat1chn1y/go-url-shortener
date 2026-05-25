package storage

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// fileStorageEntry представляет одну запись в JSON-файле.
type fileStorageEntry struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// FileStorage хранит данные в памяти и на диске.
type FileStorage struct {
	mu       sync.RWMutex
	data     map[string]string // id -> originalURL
	filePath string
}

// NewFileStorage создаёт новое файловое хранилище.
// Загружает существующие данные из файла, если он есть.
func NewFileStorage(filePath string) (*FileStorage, error) {
	fs := &FileStorage{
		data:     make(map[string]string),
		filePath: filePath,
	}
	// Загружаем данные из файла, если он существует
	if err := fs.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return fs, nil
}

// load читает данные из JSON-файла и заполняет карту.
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
		fs.data[entry.ShortURL] = entry.OriginalURL
	}
	return nil
}

// save записывает текущую карту в файл.
// Вызывается только когда mu уже заблокирована на чтение или запись.
func (fs *FileStorage) save() error {
	entries := make([]fileStorageEntry, 0, len(fs.data))
	for shortURL, originalURL := range fs.data {
		entries = append(entries, fileStorageEntry{
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
	}

	file, err := os.Create(fs.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// Save сохраняет пару id->originalURL в памяти и синхронно в файл.
func (fs *FileStorage) Save(id, originalURL string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if _, exists := fs.data[id]; exists {
		return ErrAlreadyExists
	}
	fs.data[id] = originalURL

	return fs.save()
}

// Load возвращает оригинальный URL по id.
func (fs *FileStorage) Load(id string) (string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	original, ok := fs.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return original, nil
}
