package storage

// urlEntry – внутреннее представление записи для in-memory и файлового хранилищ.
type urlEntry struct {
	originalURL string
	userID      string
	deleted     bool
}
