package cache_test

import (
	"context"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/cache"
)

func TestCacheExample(t *testing.T) {
	root := gest.NewModule(gest.ModuleConfig{
		Name: "CacheExample",
		Imports: gest.Imports(
			cache.Module(cache.Options{Global: true}),
			profilesModule(),
		),
	})
	container, err := gest.NewContainer(root)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	resolved, err := container.Resolve(gest.TokenOf[*profileService]())
	if err != nil {
		t.Fatalf("Resolve service returned error: %v", err)
	}
	service, ok := resolved.(*profileService)
	if !ok {
		t.Fatalf("service = %T, want *profileService", resolved)
	}

	profile, err := service.Get(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if profile.Name != "Ada" {
		t.Fatalf("profile name = %q, want Ada", profile.Name)
	}
	if service.loads != 1 {
		t.Fatalf("loads = %d, want 1", service.loads)
	}
	_, err = service.Get(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if service.loads != 1 {
		t.Fatalf("loads after cache hit = %d, want 1", service.loads)
	}
}

func profilesModule() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "ProfilesModule",
		Providers: gest.Providers(
			gest.Provide(newProfileService),
		),
	})
}

type profile struct {
	ID   string
	Name string
}

type profileService struct {
	cache *cache.Service
	loads int
}

func newProfileService(cache *cache.Service) *profileService {
	return &profileService{cache: cache}
}

func (s *profileService) Get(ctx context.Context, id string) (profile, error) {
	key := "profile:" + id
	var cached profile
	ok, err := s.cache.GetJSON(ctx, key, &cached)
	if err != nil || ok {
		return cached, err
	}
	s.loads++
	loaded := profile{ID: id, Name: "Ada"}
	if err := s.cache.SetJSON(ctx, key, loaded, 0); err != nil {
		return profile{}, err
	}
	return loaded, nil
}
