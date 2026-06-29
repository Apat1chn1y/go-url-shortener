package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/stretchr/testify/assert"
)

// mockStorage реализует storage.Storage для тестирования хендлеров.
type mockStorage struct {
	saveFunc       func(id, originalURL string) error
	loadFunc       func(id string) (string, error)
	pingFunc       func() error
	saveBatchFunc  func(urls map[string]string) error
	findByOrigFunc func(originalURL string) (string, error)
}

func (m *mockStorage) FindByOriginal(originalURL string) (string, error) {
	if m.findByOrigFunc != nil {
		return m.findByOrigFunc(originalURL)
	}
	return "", storage.ErrNotFound
}

func (m *mockStorage) SaveBatch(urls map[string]string) error {
	if m.saveBatchFunc != nil {
		return m.saveBatchFunc(urls)
	}
	return nil
}

func (m *mockStorage) Ping() error {
	if m.pingFunc != nil {
		return m.pingFunc()
	}
	return nil // по умолчанию успех
}

func TestShortenHandler_HandleBatchShorten(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           string
		saveBatchFunc  func(urls map[string]string) error
		expectedStatus int
		expectedBody   string
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "success",
			contentType:    "application/json",
			body:           `[{"correlation_id":"1","original_url":"https://ya.ru"},{"correlation_id":"2","original_url":"https://google.com"}]`,
			saveBatchFunc:  func(urls map[string]string) error { return nil },
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, body string) {
				var resp []struct {
					CorrelationID string `json:"correlation_id"`
					ShortURL      string `json:"short_url"`
				}
				err := json.Unmarshal([]byte(body), &resp)
				require.NoError(t, err)
				assert.Len(t, resp, 2)
				assert.Equal(t, "1", resp[0].CorrelationID)
				assert.Contains(t, resp[0].ShortURL, "http://localhost:8080/")
			},
		},
		{
			name:           "empty batch",
			contentType:    "application/json",
			body:           `[]`,
			saveBatchFunc:  nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty batch\n",
		},
		{
			name:           "invalid JSON",
			contentType:    "application/json",
			body:           `[{"correlation_id":`,
			saveBatchFunc:  nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: invalid JSON\n",
		},
		{
			name:           "missing original_url",
			contentType:    "application/json",
			body:           `[{"correlation_id":"1","original_url":""}]`,
			saveBatchFunc:  nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty original_url in batch\n",
		},
		{
			name:        "storage error",
			contentType: "application/json",
			body:        `[{"correlation_id":"1","original_url":"https://ya.ru"}]`,
			saveBatchFunc: func(urls map[string]string) error {
				return storage.ErrAlreadyExists // симулируем конфликт
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: ID already exists\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStorage{
				saveBatchFunc: tt.saveBatchFunc,
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/")

			req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			handler.HandleBatchShorten(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, rr.Body.String())
			}
			if tt.checkResponse != nil {
				tt.checkResponse(t, rr.Body.String())
			}
		})
	}
}

func (m *mockStorage) Save(id, originalURL string) error {
	if m.saveFunc != nil {
		return m.saveFunc(id, originalURL)
	}
	return nil
}

func (m *mockStorage) Load(id string) (string, error) {
	if m.loadFunc != nil {
		return m.loadFunc(id)
	}
	return "", storage.ErrNotFound
}

// TestShortenHandler_Create тестирует обработчик Create (POST /).
func TestShortenHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           string
		saveFunc       func(id, url string) error
		expectedStatus int
		expectedBody   string
		checkBody      func(t *testing.T, body string)
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "text/plain",
			body:        "https://ya.ru",
			saveFunc: func(id, url string) error {
				if id == "" || url != "https://ya.ru" {
					return errors.New("unexpected args")
				}
				return nil
			},
			expectedStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "http://localhost:8080/")
				assert.Greater(t, len(body), len("http://localhost:8080/"))
			},
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "text/plain", headers.Get("Content-Type"))
			},
		},
		{
			name:           "wrong content-type",
			contentType:    "application/json",
			body:           "https://ya.ru",
			saveFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Content-Type must be text/plain\n",
		},
		{
			name:           "empty body",
			contentType:    "text/plain",
			body:           "",
			saveFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty or invalid body\n",
		},
		{
			name:        "storage error (collision)",
			contentType: "text/plain",
			body:        "https://example.com",
			saveFunc: func(id, url string) error {
				return storage.ErrAlreadyExists
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: failed to generate unique ID after 10 attempts\n",
		},
		{
			name:        "storage other error",
			contentType: "text/plain",
			body:        "https://example.com",
			saveFunc: func(id, url string) error {
				return errors.New("database connection lost")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStorage{saveFunc: tt.saveFunc}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/")

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			handler.Create(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, rr.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rr.Body.String())
			}
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, rr.Header())
			}
		})
	}
}

// TestShortenHandler_Redirect тестирует обработчик Redirect (GET /{id}).
func TestShortenHandler_Redirect(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		loadFunc       func(id string) (string, error)
		expectedStatus int
		expectedHeader string
	}{
		{
			name: "success redirect",
			path: "/abc123",
			loadFunc: func(id string) (string, error) {
				if id != "abc123" {
					return "", errors.New("wrong id")
				}
				return "https://ya.ru", nil
			},
			expectedStatus: http.StatusTemporaryRedirect,
			expectedHeader: "https://ya.ru",
		},
		{
			name:           "empty id",
			path:           "/",
			loadFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
		},
		{
			name: "not found",
			path: "/missing",
			loadFunc: func(id string) (string, error) {
				return "", storage.ErrNotFound
			},
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStorage{loadFunc: tt.loadFunc}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/")

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.Redirect(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedHeader != "" {
				assert.Equal(t, tt.expectedHeader, rr.Header().Get("Location"))
			} else {
				assert.Empty(t, rr.Header().Get("Location"))
			}
		})
	}
}

// TestShortenHandler_HandleShortenJSON тестирует JSON-эндпоинт POST /api/shorten.
func TestShortenHandler_HandleShortenJSON(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           string
		saveFunc       func(id, url string) error
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `{"url":"https://ya.ru"}`,
			saveFunc: func(id, url string) error {
				if id == "" || url != "https://ya.ru" {
					return errors.New("unexpected args")
				}
				return nil
			},
			expectedStatus: http.StatusCreated,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "application/json", headers.Get("Content-Type"))
			},
			// тело ответа проверяем отдельно
		},
		{
			name:           "wrong content-type",
			contentType:    "text/plain",
			body:           `{"url":"https://ya.ru"}`,
			saveFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Content-Type must be application/json\n",
		},
		{
			name:           "invalid JSON",
			contentType:    "application/json",
			body:           `{"url"`,
			saveFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: invalid JSON\n",
		},
		{
			name:           "empty url field",
			contentType:    "application/json",
			body:           `{"url":""}`,
			saveFunc:       nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: url field is empty\n",
		},
		{
			name:        "storage collision",
			contentType: "application/json",
			body:        `{"url":"https://example.com"}`,
			saveFunc: func(id, url string) error {
				return storage.ErrAlreadyExists
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: failed to generate unique ID after 10 attempts\n",
		},
		{
			name:        "storage other error",
			contentType: "application/json",
			body:        `{"url":"https://example.com"}`,
			saveFunc: func(id, url string) error {
				return errors.New("database connection lost")
			},
			expectedStatus: http.StatusInternalServerError, // было 400
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:           "content-type with charset",
			contentType:    "application/json; charset=utf-8",
			body:           `{"url":"https://ya.ru"}`,
			saveFunc:       func(id, url string) error { return nil },
			expectedStatus: http.StatusCreated,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "application/json", headers.Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStorage{saveFunc: tt.saveFunc}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/")

			req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			handler.HandleShortenJSON(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, rr.Body.String())
			}
			if tt.checkHeaders != nil {
				tt.checkHeaders(t, rr.Header())
			}
			// Для успешного случая проверяем структуру JSON
			if tt.expectedStatus == http.StatusCreated {
				var resp shortenResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp.Result, "http://localhost:8080/")
				assert.NotEmpty(t, resp.Result)
			}
		})
	}
}

func TestShortenHandler_Ping(t *testing.T) {
	mockStore := &mockStorage{
		pingFunc: func() error { return nil },
	}
	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/")

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestShortenHandler_Ping_Error(t *testing.T) {
	mockStore := &mockStorage{
		pingFunc: func() error { return errors.New("db down") },
	}
	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/")

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Internal Server Error")
}

// TestShortenHandler_Create_Duplicate
func TestShortenHandler_Create_Duplicate(t *testing.T) {
	mockStore := &mockStorage{
		findByOrigFunc: func(originalURL string) (string, error) {
			if originalURL == "https://ya.ru" {
				return "abc123", nil
			}
			return "", storage.ErrNotFound
		},
	}
	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/")

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "http://localhost:8080/abc123", rr.Body.String())
	assert.Equal(t, "text/plain", rr.Result().Header.Get("Content-Type"))
}

// TestShortenHandler_HandleShortenJSON_Duplicate
func TestShortenHandler_HandleShortenJSON_Duplicate(t *testing.T) {
	mockStore := &mockStorage{
		findByOrigFunc: func(originalURL string) (string, error) {
			if originalURL == "https://ya.ru" {
				return "abc123", nil
			}
			return "", storage.ErrNotFound
		},
	}
	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/")

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(`{"url":"https://ya.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleShortenJSON(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "application/json", rr.Result().Header.Get("Content-Type"))

	var resp shortenResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/abc123", resp.Result)
}
