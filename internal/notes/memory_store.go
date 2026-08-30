package notes

import "sync"

// MemoryStore is a deterministic in-memory session used when embedding the
// app without executable persistence wiring.
type MemoryStore struct {
	mu       sync.Mutex
	Text     string
	ReadOnly bool
	LoadErr  error
	SaveErr  error
	Closed   bool
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (s *MemoryStore) Load() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Text, s.ReadOnly, s.LoadErr
}

func (s *MemoryStore) Save(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ReadOnly {
		return ErrReadOnly
	}
	if s.SaveErr != nil {
		return s.SaveErr
	}
	s.Text = text
	return nil
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Closed = true
	return nil
}
