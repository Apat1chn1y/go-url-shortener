package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	storagemocks "github.com/Apat1chn1y/go-url-shortener.git/internal/mocks/storage"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestShortenHandler_Create
func TestShortenHandler_Create(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		contentType    string
		body           string
		setupMock      func(*storagemocks.MockStorage)
		expectedStatus int
		expectedBody   string
		checkBody      func(t *testing.T, body string)
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "text/plain",
			body:        "https://ya.ru",
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", TestUserID).Return(nil).Once()
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
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://example.com", TestUserID).Return(storage.ErrAlreadyExists).Times(10)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "storage other error",
			contentType: "text/plain",
			body:        "https://example.com",
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://example.com", TestUserID).Return(errors.New("database connection lost")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewMockStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

			req := NewRequestWithUserID(http.MethodPost, "/", []byte(tt.body))
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

// TestShortenHandler_Redirect – без изменений, кроме логирования (уже передаётся логгер)
func TestShortenHandler_Redirect(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		path           string
		setupMock      func(*storagemocks.MockStorage)
		expectedStatus int
		expectedHeader string
	}{
		{
			name: "success redirect",
			path: "/abc123",
			setupMock: func(m *storagemocks.MockStorage) {
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
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().Load("missing").Return("", storage.ErrNotFound).Once()
			},
			expectedStatus: http.StatusBadRequest,
			expectedHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewMockStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

			req := NewRequestWithUserID(http.MethodGet, tt.path, nil)
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

// TestShortenHandler_HandleShortenJSON – аналогично
func TestShortenHandler_HandleShortenJSON(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		contentType    string
		body           string
		setupMock      func(*storagemocks.MockStorage)
		expectedStatus int
		expectedBody   string
		checkHeaders   func(t *testing.T, headers http.Header)
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `{"url":"https://ya.ru"}`,
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", TestUserID).Return(nil).Once()
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
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://example.com", TestUserID).Return(storage.ErrAlreadyExists).Times(10)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "storage other error",
			contentType: "application/json",
			body:        `{"url":"https://example.com"}`,
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://example.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://example.com", TestUserID).Return(errors.New("database connection lost")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:        "content-type with charset",
			contentType: "application/json; charset=utf-8",
			body:        `{"url":"https://ya.ru"}`,
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveForUser(mock.Anything, "https://ya.ru", TestUserID).Return(nil).Once()
			},
			expectedStatus: http.StatusCreated,
			checkHeaders: func(t *testing.T, headers http.Header) {
				assert.Contains(t, headers.Get("Content-Type"), "application/json")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewMockStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

			req := NewRequestWithUserID(http.MethodPost, "/api/shorten", []byte(tt.body))
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

// TestShortenHandler_Ping – без изменений (уже передаётся логгер)
func TestShortenHandler_Ping(t *testing.T) {
	logger := zerolog.Nop()
	mockStore := storagemocks.NewMockStorage(t)
	mockStore.EXPECT().Ping().Return(nil).Once()

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestShortenHandler_Ping_Error(t *testing.T) {
	logger := zerolog.Nop()
	mockStore := storagemocks.NewMockStorage(t)
	mockStore.EXPECT().Ping().Return(errors.New("db down")).Once()

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rr := httptest.NewRecorder()
	handler.Ping(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Internal Server Error")
}

// TestShortenHandler_Create_Duplicate – проверка 409
func TestShortenHandler_Create_Duplicate(t *testing.T) {
	logger := zerolog.Nop()
	mockStore := storagemocks.NewMockStorage(t)
	mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("abc123", nil).Once()
	// Save не вызывается

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

	req := NewRequestWithUserID(http.MethodPost, "/", []byte("https://ya.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	handler.Create(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "http://localhost:8080/abc123", rr.Body.String())
	assert.Contains(t, rr.Result().Header.Get("Content-Type"), "text/plain")
}

func TestShortenHandler_HandleShortenJSON_Duplicate(t *testing.T) {
	logger := zerolog.Nop()
	mockStore := storagemocks.NewMockStorage(t)
	mockStore.EXPECT().FindByOriginal("https://ya.ru").Return("abc123", nil).Once()

	shortener := service.NewShortener(mockStore)
	handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

	req := NewRequestWithUserID(http.MethodPost, "/api/shorten", []byte(`{"url":"https://ya.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.HandleShortenJSON(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Result().Header.Get("Content-Type"), "application/json")

	var resp shortenResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/abc123", resp.Result)
}

// TestShortenHandler_HandleBatchShorten – обновлён для использования SaveBatchForUser
func TestShortenHandler_HandleBatchShorten(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name           string
		contentType    string
		body           string
		setupMock      func(*storagemocks.MockStorage)
		expectedStatus int
		expectedBody   string
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:        "success",
			contentType: "application/json",
			body:        `[{"correlation_id":"1","original_url":"https://ya.ru"},{"correlation_id":"2","original_url":"https://google.com"}]`,
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().FindByOriginal("https://google.com").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveBatchForUser(mock.Anything, TestUserID).Return(nil).Once()
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
				// Проверяем порядок (первый должен быть с correlation_id "1")
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
			name:        "storage error (SaveBatchForUser fails)",
			contentType: "application/json",
			body:        `[{"correlation_id":"1","original_url":"https://ya.ru"}]`,
			setupMock: func(m *storagemocks.MockStorage) {
				m.EXPECT().FindByOriginal("https://ya.ru").Return("", storage.ErrNotFound).Once()
				m.EXPECT().SaveBatchForUser(mock.Anything, TestUserID).Return(storage.ErrAlreadyExists).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := storagemocks.NewMockStorage(t)
			if tt.setupMock != nil {
				tt.setupMock(mockStore)
			}
			shortener := service.NewShortener(mockStore)
			handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

			req := NewRequestWithUserID(http.MethodPost, "/api/shorten/batch", []byte(tt.body))
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

// TestShortenHandler_GetUserURLs – новый тест для эндпоинта /api/user/urls
func TestShortenHandler_GetUserURLs(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success with URLs", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		expectedUserURLs := []storage.UserURL{
			{ID: "abc123", ShortURL: "", OriginalURL: "https://ya.ru"},
			{ID: "def456", ShortURL: "", OriginalURL: "https://google.com"},
		}
		mockStore.EXPECT().GetUserURLs(TestUserID).Return(expectedUserURLs, nil).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

		req := NewRequestWithUserID(http.MethodGet, "/api/user/urls", nil)
		rr := httptest.NewRecorder()
		handler.GetUserURLs(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

		var resp []struct {
			ShortURL    string `json:"short_url"`
			OriginalURL string `json:"original_url"`
		}
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp, 2)
		assert.Equal(t, "http://localhost:8080/abc123", resp[0].ShortURL)
		assert.Equal(t, "https://ya.ru", resp[0].OriginalURL)
		assert.Equal(t, "http://localhost:8080/def456", resp[1].ShortURL)
		assert.Equal(t, "https://google.com", resp[1].OriginalURL)
	})

	t.Run("no URLs -> 204", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		mockStore.EXPECT().GetUserURLs(TestUserID).Return([]storage.UserURL{}, nil).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

		req := NewRequestWithUserID(http.MethodGet, "/api/user/urls", nil)
		rr := httptest.NewRecorder()
		handler.GetUserURLs(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("no userID -> 401", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		// Не вызываем GetUserURLs
		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil) // без контекста
		rr := httptest.NewRecorder()
		handler.GetUserURLs(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "Unauthorized")
	})

	t.Run("storage error -> 500", func(t *testing.T) {
		mockStore := storagemocks.NewMockStorage(t)
		mockStore.EXPECT().GetUserURLs(TestUserID).Return(nil, errors.New("db error")).Once()

		shortener := service.NewShortener(mockStore)
		handler := NewShortenHandler(shortener, "http://localhost:8080/", logger)

		req := NewRequestWithUserID(http.MethodGet, "/api/user/urls", nil)
		rr := httptest.NewRecorder()
		handler.GetUserURLs(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "Internal Server Error")
	})
}
