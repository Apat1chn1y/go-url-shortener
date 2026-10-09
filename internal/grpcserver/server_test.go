package grpcserver_test

import (
	"context"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/Apat1chn1y/go-url-shortener.git/internal/auth"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/grpcserver"
	pb "github.com/Apat1chn1y/go-url-shortener.git/internal/proto"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/service"
	"github.com/Apat1chn1y/go-url-shortener.git/internal/storage"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1024 * 1024

// setupTestServer поднимает gRPC-сервер в памяти (bufconn) и возвращает клиента,
// общий сервис бизнес-логики и ключ подписи.
func setupTestServer(t *testing.T) (pb.ShortenerServiceClient, *service.Shortener, []byte) {
	t.Helper()

	authKey := []byte("test-auth-key")
	store := storage.NewInMemoryStorage()
	shortener := service.NewShortener(store)

	handler := grpcserver.NewShortenerServer(shortener, "http://localhost:8080/", zerolog.Nop(), authKey)

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterShortenerServiceServer(srv, handler)

	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewShortenerServiceClient(conn), shortener, authKey
}

// authContext кладёт в metadata authorization=<userID>|<sig>, где подпись
// построена тем же кодом, что и HTTP-кука (auth.SetUserCookie).
func authContext(t *testing.T, key []byte, userID string) context.Context {
	t.Helper()
	rr := httptest.NewRecorder()
	auth.SetUserCookie(rr, userID, key)
	cookies := rr.Result().Cookies()
	require.NotEmpty(t, cookies)

	md := metadata.Pairs("authorization", cookies[0].Value)
	return metadata.NewOutgoingContext(context.Background(), md)
}

func TestShortenURL_Success(t *testing.T) {
	client, _, _ := setupTestServer(t)

	resp, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: "https://ya.ru"})
	require.NoError(t, err)
	assert.Contains(t, resp.GetResult(), "http://localhost:8080/")
}

func TestShortenURL_EmptyURL(t *testing.T) {
	client, _, _ := setupTestServer(t)

	_, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: ""})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestExpandURL_Success(t *testing.T) {
	client, shortener, _ := setupTestServer(t)

	shortURL, err := shortener.Create("https://ya.ru", "http://localhost:8080/", "")
	require.NoError(t, err)
	id := shortURL[len("http://localhost:8080/"):]

	resp, err := client.ExpandURL(context.Background(), &pb.URLExpandRequest{Id: id})
	require.NoError(t, err)
	assert.Equal(t, "https://ya.ru", resp.GetResult())
}

func TestExpandURL_NotFound(t *testing.T) {
	client, _, _ := setupTestServer(t)

	_, err := client.ExpandURL(context.Background(), &pb.URLExpandRequest{Id: "missing"})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestListUserURLs_Unauthenticated(t *testing.T) {
	client, _, _ := setupTestServer(t)

	_, err := client.ListUserURLs(context.Background(), &emptypb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestListUserURLs_Success(t *testing.T) {
	client, shortener, key := setupTestServer(t)

	const userID = "test-user-123"
	_, err := shortener.Create("https://ya.ru", "http://localhost:8080/", userID)
	require.NoError(t, err)

	ctx := authContext(t, key, userID)

	resp, err := client.ListUserURLs(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, resp.GetUrl(), 1)
	assert.Equal(t, "https://ya.ru", resp.GetUrl()[0].GetOriginalUrl())
	assert.Contains(t, resp.GetUrl()[0].GetShortUrl(), "http://localhost:8080/")
}

func TestShortenURL_AlreadyExists(t *testing.T) {
	client, _, _ := setupTestServer(t)

	// Первый раз — реальное создание.
	first, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: "https://ya.ru"})
	require.NoError(t, err)
	assert.False(t, first.GetAlreadyExists())
	assert.NotEmpty(t, first.GetResult())

	// Второй раз — уже существует: получаем тот же URL и флаг already_exists,
	// статус OK (никакой status.Error клиент не увидит).
	second, err := client.ShortenURL(context.Background(), &pb.URLShortenRequest{Url: "https://ya.ru"})
	require.NoError(t, err)
	assert.True(t, second.GetAlreadyExists())
	assert.Equal(t, first.GetResult(), second.GetResult())
}
