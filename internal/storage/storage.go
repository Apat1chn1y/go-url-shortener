// Package storage предоставляет интерфейс и ошибки для хранилища URL.
package storage

import "errors"

var (
	ErrNotFound             = errors.New("URL not found")
	ErrAlreadyExists        = errors.New("ID already exists")
	ErrOriginalURLDuplicate = errors.New("original URL already exists")
	ErrGone                 = errors.New("URL has been deleted")
	ErrForbidden            = errors.New("access denied")
)

// UserURL содержит данные об URL для ответа пользователю.
type UserURL struct {
	ID          string `json:"-"` // внутренний идентификатор (не возвращается в JSON)
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	DeletedFlag bool   `json:"-"` // флаг удаления (не возвращается в JSON)
}

// Storage определяет контракт для хранилища.
type Storage interface {
	// Save – устаревший метод без userID, оставлен для обратной совместимости.
	Save(id, originalURL string) error

	// SaveForUser сохраняет URL с привязкой к пользователю.
	SaveForUser(id, originalURL, userID string) error

	// Load возвращает оригинальный URL по id.
	Load(id string) (string, error)

	// Ping проверяет доступность хранилища.
	Ping() error

	// SaveBatch сохраняет множество записей (без привязки к пользователю, используется в батче).
	SaveBatch(urls map[string]string) error

	// SaveBatchForUser сохраняет множество записей с привязкой к пользователю.
	SaveBatchForUser(urls map[string]string, userID string) error

	// FindByOriginal возвращает id по оригинальному URL.
	FindByOriginal(originalURL string) (string, error)

	// GetUserURLs возвращает все URL, принадлежащие пользователю.
	GetUserURLs(userID string) ([]UserURL, error)

	// DeleteUserURLs помечает URL как удалённые для данного пользователя.
	DeleteUserURLs(userID string, ids []string) error
}
