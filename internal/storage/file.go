package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// fileStorageEntry представляет одну запись в JSON-файле согласно спецификации.
type fileStorageEntry struct {
	UUID        string `json:"uuid"`
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

func (fs *FileStorage) Ping() error {
	// Проверяем, можем ли открыть файл на чтение
	_, err := os.Open(fs.filePath)
	return err
}

// SaveBatch атомарно сохраняет несколько пар id->originalURL в память и файл.
func (fs *FileStorage) SaveBatch(urls map[string]string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for id, originalURL := range urls {
		if _, exists := fs.data[id]; exists {
			return ErrAlreadyExists
		}
		fs.data[id] = originalURL
	}
	return fs.save() // перезаписывает файл атомарно
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
// Вызывается внутри Save под mu.Lock().
func (fs *FileStorage) save() error {

	entries := make([]fileStorageEntry, 0, len(fs.data))
	for shortURL, originalURL := range fs.data {
		entries = append(entries, fileStorageEntry{
			UUID:        uuid.New().String(),
			ShortURL:    shortURL,
			OriginalURL: originalURL,
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

// Save сохраняет пару id->originalURL.
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
