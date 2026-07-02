package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"

	storagemocks "github.com/Apat1chn1y/go-url-shortener.git/internal/mocks/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestShortenHandler_Create тестирует обработчик Create (POST /).
func TestShortenHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           string
		setupMock      func(*storagemocks.Storage)
		expectedStatus int
		expectedBody   string
		checkBody      func(t *testing.T, body string)
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "text/plain",
			body:        "https://ya.ru",
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://ya.ru").Return(nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "http://localhost:8080/")
				assert.Greater(t, len(body), len("http://localhost:8080/"))
			},
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Contains(t, headers.Get("Content-Type"), "text/plain")
			},
		},
		{
			name:           "wrong content-type",
			contentType:    "application/json",
			body:           "https://ya.ru",
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Content-Type must be text/plain\n",
		},
		{
			name:           "empty body",
			contentType:    "text/plain",
			body:           "",
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty or invalid body\n",
		},
		{
			name:        "storage error (collision)",
			contentType: "text/plain",
			body:        "https://example.com",
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://example.com").Return(storage.ErrAlreadyExists).Times(10)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "storage other error",
			contentType: "text/plain",
			body:        "https://example.com",
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://example.com").Return(errors.New("database connection lost")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

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
		setupMock      func(*storagemocks.Storage)
		expectedStatus int
		expectedHeader string
	}{
		{
			name: "success redirect",
			path: "/abc123",
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().Load("abc123").Return("https://ya.ru", nil).Once()
			},
			expectedStatus: http.StatusTemporaryRedirect,
			expectedHeader: "https://ya.ru",
		},
		{
			name:           "empty id",
			path:           "/",
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
		},
		{
			name: "not found",
			path: "/missing",
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().Load("missing").Return("", storage.ErrNotFound).Once()
			},
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

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
		setupMock      func(*storagemocks.Storage)
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `{"url":"https://ya.ru"}`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://ya.ru").Return(nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Contains(t, headers.Get("Content-Type"), "application/json")
			},
		},
		{
			name:           "wrong content-type",
			contentType:    "text/plain",
			body:           `{"url":"https://ya.ru"}`,
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: Content-Type must be application/json\n",
		},
		{
			name:           "invalid JSON",
			contentType:    "application/json",
			body:           `{"url"`,
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: invalid JSON\n",
		},
		{
			name:           "empty url field",
			contentType:    "application/json",
			body:           `{"url":""}`,
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: url field is empty\n",
		},
		{
			name:        "storage collision",
			contentType: "application/json",
			body:        `{"url":"https://example.com"}`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://example.com").Return(storage.ErrAlreadyExists).Times(10)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "storage other error",
			contentType: "application/json",
			body:        `{"url":"https://example.com"}`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://example.com").Return(errors.New("database connection lost")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "content-type with charset",
			contentType: "application/json; charset=utf-8",
			body:        `{"url":"https://ya.ru"}`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().Save(mock.Anything, "https://ya.ru").Return(nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Equal(t, "application/json", headers.Get("Content-Type"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

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

// TestShortenHandler_Ping тестирует эндпоинт /ping при успешном соединении.
func TestShortenHandler_Ping(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)
	mockStore.EXPECT().Ping().Return(nil).Once()

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

// TestShortenHandler_Ping_Error тестирует эндпоинт /ping при ошибке соединения.
func TestShortenHandler_Ping_Error(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)
	mockStore.EXPECT().Ping().Return(errors.New("db down")).Once()

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Internal Server Error")
}

// TestShortenHandler_Create_Duplicate проверяет возврат 409 Conflict для уже существующего URL.
func TestShortenHandler_Create_Duplicate(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)
	mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("abc123", nil).Once()
	// Save не вызывается

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "http://localhost:8080/abc123", rr.Body.String())
	// Проверяем, что Content-Type содержит "text/plain" (может быть с charset)
	assert.Contains(t, rr.Result().Header.Get("Content-Type"), "text/plain")
}

// TestShortenHandler_HandleShortenJSON_Duplicate проверяет возврат 409 для JSON эндпоинта.
func TestShortenHandler_HandleShortenJSON_Duplicate(t *testing.T) {
	mockStore := storagemocks.NewStorage(t)
	mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("abc123", nil).Once()
	// Save не вызывается

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(`{"url":"https://ya.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleShortenJSON(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	// Проверяем, что Content-Type содержит "application/json"
	assert.Contains(t, rr.Result().Header.Get("Content-Type"), "application/json")

	var resp shortenResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/abc123", resp.Result)
}

// TestShortenHandler_HandleBatchShorten тестирует батчевый эндпоинт POST /api/shorten/batch.
func TestShortenHandler_HandleBatchShorten(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		body           string
		setupMock      func(*storagemocks.Storage)
		expectedStatus int
		expectedBody   string
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `[{"correlation_id":"1","original_url":"https://ya.ru"},{"correlation_id":"2","original_url":"https://google.com"}]`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().FindByOriginal("https://google.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveBatch(mock.Anything).Return(nil).Once()
			},
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
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty batch\n",
		},
		{
			name:           "invalid JSON",
			contentType:    "application/json",
			body:           `[{"correlation_id":`,
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: invalid JSON\n",
		},
		{
			name:           "missing original_url",
			contentType:    "application/json",
			body:           `[{"correlation_id":"1","original_url":""}]`,
			setupMock:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Bad Request: empty original_url in batch\n",
		},
		{
			name:        "storage error (SaveBatch fails)",
			contentType: "application/json",
			body:        `[{"correlation_id":"1","original_url":"https://ya.ru"}]`,
			setupMock: func(m *storagemocks.Storage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveBatch(mock.Anything).Return(storage.ErrAlreadyExists).Once()
			},
			// Ошибка SaveBatch (например, коллизия) считается внутренней, поэтому 500
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", zerolog.Nop())

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
