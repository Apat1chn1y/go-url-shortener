// Package audit предоставляет функциональность аудита запросов.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// Event представляет событие аудита.
type Event struct {
	Ts     int64  `json:"ts"`      // unix timestamp
	Action string `json:"action"`  // "shorten" или "follow"
	UserID string `json:"user_id"` // идентификатор пользователя (если есть)
	URL    string `json:"url"`     // оригинальный (не сокращённый) URL
}

// Writer определяет интерфейс для отправки события аудита с контекстом.
type Writer interface {
	Write(ctx context.Context, event Event) error
}

// writerWorker – внутренний воркер для одного писателя.
type writerWorker struct {
	writer Writer
	ch     chan Event    // буферизованный канал событий
	done   chan struct{} // сигнал остановки
}

// Manager управляет писателями аудита (наблюдателями).
type Manager struct {
	mu      sync.Mutex
	workers []*writerWorker
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	sync    bool // если true, отправка событий синхронная (для тестов)
}

// NewManager создаёт новый менеджер аудита с контекстом для отмены.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetSyncMode включает синхронную отправку событий (используется в тестах).
func (m *Manager) SetSyncMode(sync bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sync = sync
}

// AddWriter добавляет писателя и запускает фоновую горутину для его обработки.
func (m *Manager) AddWriter(w Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Event, 1000) // буфер для предотвращения блокировки основного потока
	done := make(chan struct{})
	worker := &writerWorker{
		writer: w,
		ch:     ch,
		done:   done,
	}
	m.workers = append(m.workers, worker)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case event := <-ch:
				_ = worker.writer.Write(m.ctx, event)
			case <-done:
				return
			}
		}
	}()
}

// Notify отправляет событие всем зарегистрированным писателям.
// Если включён синхронный режим – вызывает Write напрямую, иначе отправляет в каналы воркеров.
func (m *Manager) Notify(event Event) {
	m.mu.Lock()
	workers := make([]*writerWorker, len(m.workers))
	copy(workers, m.workers)
	syncMode := m.sync
	m.mu.Unlock()

	if len(workers) == 0 {
		return
	}

	if syncMode {
		for _, w := range workers {
			_ = w.writer.Write(m.ctx, event)
		}
		return
	}

	// Асинхронный режим – отправляем в каналы воркеров
	for _, w := range workers {
		select {
		case w.ch <- event:
		default:
			// канал полон – пропускаем
		}
	}
}

// Close останавливает все фоновые горутины и ожидает их завершения.
func (m *Manager) Close() {
	m.cancel()
	m.mu.Lock()
	workers := m.workers
	m.workers = nil
	m.mu.Unlock()

	for _, w := range workers {
		close(w.done)
	}
	m.wg.Wait()
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

// Write записывает событие в файл.
func (w *FileWriter) Write(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.file.Write(data)
	return err
}

// Close закрывает файл.
func (w *FileWriter) Close() error {
	return w.file.Close()
}

// HTTPWriter реализует отправку на удалённый сервер методом POST с автоматическими ретраями.
type HTTPWriter struct {
	url    string
	client *retryablehttp.Client
}

// NewHTTPWriter создаёт HTTP писатель с клиентом, поддерживающим ретраи.
func NewHTTPWriter(url string) *HTTPWriter {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 500 * time.Millisecond
	client.RetryWaitMax = 5 * time.Second
	return &HTTPWriter{
		url:    url,
		client: client,
	}
}

// Write отправляет событие на удалённый сервер с автоматическими ретраями.
func (w *HTTPWriter) Write(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("audit http request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
