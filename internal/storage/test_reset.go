package storage

// generate:reset
type TestReset struct {
	ID    int
	Name  string
	Tags  []string
	Data  map[string]string
	Child *TestReset
}
