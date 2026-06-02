package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/cache/adapters/memory"
)

// Options configures the optional cache module.
type Options struct {
	Global bool
	Store  Store
}

// Store is the adapter contract used by Service.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Service provides simple cache operations through DI.
type Service struct {
	store Store
}

// Module returns a Gest module that provides *cache.Service through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name:   "CacheModule",
		Global: options.Global,
		Providers: gest.Providers(
			gest.Provide(func() *Service {
				return NewService(options)
			}),
		),
	})
}

// NewService creates a cache service using the configured store or memory by default.
func NewService(options Options) *Service {
	store := options.Store
	if store == nil {
		store = memory.NewStore()
	}
	return &Service{store: store}
}

// Get returns a cached byte value and whether it was present.
func (s *Service) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := validateKey(key); err != nil {
		return nil, false, err
	}
	if s == nil || s.store == nil {
		return nil, false, fmt.Errorf("CACHE_INVALID_SERVICE: cache store is nil")
	}
	return s.store.Get(ctx, key)
}

// Set stores a byte value. A ttl of zero means no expiration.
func (s *Service) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if ttl < 0 {
		return fmt.Errorf("CACHE_INVALID_TTL: ttl must be zero or positive")
	}
	if s == nil || s.store == nil {
		return fmt.Errorf("CACHE_INVALID_SERVICE: cache store is nil")
	}
	return s.store.Set(ctx, key, append([]byte(nil), value...), ttl)
}

// Delete removes a cached value. Missing keys are ignored.
func (s *Service) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if s == nil || s.store == nil {
		return fmt.Errorf("CACHE_INVALID_SERVICE: cache store is nil")
	}
	return s.store.Delete(ctx, key)
}

// GetJSON decodes a JSON cached value into target.
func (s *Service) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	if target == nil {
		return false, fmt.Errorf("CACHE_INVALID_TARGET: JSON target must not be nil")
	}
	value, ok, err := s.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(value, target); err != nil {
		return true, fmt.Errorf("CACHE_JSON_DECODE_FAILED: %w", err)
	}
	return true, nil
}

// SetJSON encodes and stores a JSON cached value.
func (s *Service) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("CACHE_JSON_ENCODE_FAILED: %w", err)
	}
	return s.Set(ctx, key, encoded, ttl)
}

func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("CACHE_INVALID_KEY: key must not be empty")
	}
	return nil
}
