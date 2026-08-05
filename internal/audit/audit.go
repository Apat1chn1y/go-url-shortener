// Package audit предоставляет функциональность аудита запросов.
package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// Event представляет событие аудита.
type Event struct {
	Ts     int64  `json:"ts"`      // unix timestamp
	Action string `json:"action"`  // "shorten" или "follow"
	UserID string `json:"user_id"` // идентификатор пользователя (если есть)
	URL    string `json:"url"`     // оригинальный (не сокращённый) URL
}

// Writer определяет интерфейс для отправки события аудита.
type Writer interface {
	Write(event Event) error
}

// Manager управляет писателями аудита (наблюдателями).
type Manager struct {
	mu      sync.Mutex
	writers []Writer
}

// NewManager создаёт новый менеджер аудита.
func NewManager() *Manager {
	return &Manager{
		writers: []Writer{},
	}
}

// AddWriter добавляет писателя.
func (m *Manager) AddWriter(w Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writers = append(m.writers, w)
}

// Notify отправляет событие всем зарегистрированным писателям (асинхронно).
func (m *Manager) Notify(event Event) {
	m.mu.Lock()
	writers := make([]Writer, len(m.writers))
	copy(writers, m.writers)
	m.mu.Unlock()

	if len(writers) == 0 {
		return
	}

	// Асинхронно отправляем каждому писателю
	for _, w := range writers {
		go func(w Writer) {
			_ = w.Write(event) // игнорируем ошибки, чтобы не нарушать работу сервиса
		}(w)
	}
}

// FileWriter реализует запись в файл.
type FileWriter struct {
	file *os.File
	mu   sync.Mutex
}

// NewFileWriter создаёт файловый писатель.
func NewFileWriter(path string) (*FileWriter, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &FileWriter{file: f}, nil
}

// Write записывает событие в файл (одна строка JSON).
func (w *FileWriter) Write(event Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = w.file.Write(append(data, '\n'))
	return err
}

// Close закрывает файл (если необходимо).
func (w *FileWriter) Close() error {
	return w.file.Close()
}

// HTTPWriter реализует отправку на удалённый сервер методом POST.
type HTTPWriter struct {
	url    string
	client *http.Client
}

// NewHTTPWriter создаёт HTTP писатель.
func NewHTTPWriter(url string) *HTTPWriter {
	return &HTTPWriter{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Write отправляет событие на удалённый сервер.
func (w *HTTPWriter) Write(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", w.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
