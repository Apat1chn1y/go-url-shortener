// Package grpcserver предоставляет gRPC-реализацию сервиса сокращения URL.
// Хендлеры являются тонкими фасадами к общей бизнес-логике service.Shortener,
// как и HTTP-хендлеры.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/auth"
	pb "github.com/Apat1chn1y/go-url-shortener.git/internal/proto"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// metadataAuthorizationKey — ключ metadata, в котором передаются
// авторизационные данные (значение формата "<userID>|<signature>").
const metadataAuthorizationKey = "authorization"

// ErrUnauthenticated возвращается, если metadata authorization отсутствует
// или содержит невалидную подпись.
var ErrUnauthenticated = errors.New("unauthorized")

// URLShortener — узкий интерфейс бизнес-логики, объявленный в пакете-потребителе.
// Содержит только те методы, которые реально использует gRPC-слой, что упрощает
// тестирование на моках и развязывает пакеты.
type URLShortener interface {
	Create(originalURL, baseURL, userID string) (string, error)
	Get(id string) (string, error)
	GetUserURLs(userID string) ([]storage.UserURL, error)
}

// ShortenerServer реализует gRPC-сервис ShortenerService.
type ShortenerServer struct {
	pb.UnimplementedShortenerServiceServer
	shortener URLShortener
	baseURL   string
	logger    zerolog.Logger
	authKey   []byte
}

// NewShortenerServer создаёт gRPC-хендлер с общим сервисом бизнес-логики.
func NewShortenerServer(shortener URLShortener, baseURL string, logger zerolog.Logger, authKey []byte) *ShortenerServer {
	return &ShortenerServer{
		shortener: shortener,
		baseURL:   baseURL,
		logger:    logger,
		authKey:   authKey,
	}
}

// userIDFromContext извлекает userID из metadata authorization.
// Возвращает ErrUnauthenticated, если metadata отсутствует или подпись невалидна.
// Решение о логировании и о том, как реагировать на ошибку, принимает вызывающий.
func (s *ShortenerServer) userIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", ErrUnauthenticated
	}
	vals := md.Get(metadataAuthorizationKey)
	if len(vals) == 0 {
		return "", ErrUnauthenticated
	}
	userID, err := auth.ParseUserID(vals[0], s.authKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	return userID, nil
}

// ShortenURL обрабатывает gRPC-запрос ShortenURL (аналог POST /api/shorten).
// При конфликте (URL уже сокращён) возвращает статус OK, результат и
// already_exists=true — клиент сам решает, считать это ошибкой или нет.
func (s *ShortenerServer) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url field is empty")
	}

	// Авторизация не обязательна: без metadata работаем от анонимного пользователя.
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.Debug().Err(err).Msg("gRPC shorten without valid auth")
		userID = ""
	}

	result, err := s.shortener.Create(req.GetUrl(), s.baseURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrEmptyURL) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, service.ErrURLAlreadyExists) {
			// Конфликт — не ошибка транспорта: возвращаем существующий URL
			// и флаг already_exists, чтобы клиент мог отличить создание от повтора.
			return &pb.URLShortenResponse{Result: result, AlreadyExists: true}, nil
		}
		if errors.Is(err, service.ErrMaxAttemptsExceeded) {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.URLShortenResponse{Result: result}, nil
}

// ExpandURL обрабатывает gRPC-запрос ExpandURL (аналог GET /<id>).
func (s *ShortenerServer) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id field is empty")
	}
	originalURL, err := s.shortener.Get(req.GetId())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "URL not found")
		}
		if errors.Is(err, storage.ErrGone) {
			return nil, status.Error(codes.NotFound, "URL has been deleted")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.URLExpandResponse{Result: originalURL}, nil
}

// ListUserURLs обрабатывает gRPC-запрос ListUserURLs (аналог GET /api/user/urls).
// Требует валидного userID в metadata authorization.
func (s *ShortenerServer) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Msg("gRPC ListUserURLs unauthorized")
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userURLs, err := s.shortener.GetUserURLs(userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &pb.UserURLsResponse{
		Url: make([]*pb.URLData, 0, len(userURLs)),
	}
	for _, u := range userURLs {
		resp.Url = append(resp.Url, &pb.URLData{
			ShortUrl:    s.baseURL + u.ID,
			OriginalUrl: u.OriginalURL,
		})
	}
	return resp, nil
}

// New создаёт grpc.Server с зарегистрированным сервисом.
// Если enableTLS=true, используются переданные certFile/keyFile.
func New(handler *ShortenerServer, enableTLS bool, certFile, keyFile string) (*grpc.Server, error) {
	var opts []grpc.ServerOption
	if enableTLS {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}
	srv := grpc.NewServer(opts...)
	pb.RegisterShortenerServiceServer(srv, handler)
	return srv, nil
}

// Listen создаёт TCP-listener по заданному адресу.
func Listen(addr string) (net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return lis, nil
}
