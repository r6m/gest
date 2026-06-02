package memory

import (
	"context"
	"sync"
	"time"
)

// Store is an in-process cache store suitable for tests and local development.
type Store struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

// NewStore creates an empty in-memory store.
func NewStore() *Store {
	return &Store{
		entries: make(map[string]entry),
		now:     time.Now,
	}
}

// Get returns a cached byte value and whether it was present.
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	s.mu.RLock()
	item, ok := s.entries[key]
	if !ok {
		s.mu.RUnlock()
		return nil, false, nil
	}
	expired := !item.expiresAt.IsZero() && !s.now().Before(item.expiresAt)
	if expired {
		s.mu.RUnlock()
		s.mu.Lock()
		if current, stillPresent := s.entries[key]; stillPresent && current.expiresAt.Equal(item.expiresAt) {
			delete(s.entries, key)
		}
		s.mu.Unlock()
		return nil, false, nil
	}
	value := append([]byte(nil), item.value...)
	s.mu.RUnlock()
	return value, true, nil
}

// Set stores a byte value. A ttl of zero means no expiration.
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	item := entry{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = s.now().Add(ttl)
	}
	s.mu.Lock()
	s.entries[key] = item
	s.mu.Unlock()
	return nil
}

// Delete removes a cached value. Missing keys are ignored.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
	return nil
}
