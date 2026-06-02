package gest

import (
	"context"
)

// OnModuleInit runs after eager providers/controllers are initialized and before routes are registered.
type OnModuleInit interface {
	OnModuleInit(ctx context.Context) error
}

// OnApplicationBootstrap runs after routes and OpenAPI metadata are registered.
type OnApplicationBootstrap interface {
	OnApplicationBootstrap(ctx context.Context) error
}

// BeforeApplicationShutdown runs at the start of app shutdown.
type BeforeApplicationShutdown interface {
	BeforeApplicationShutdown(ctx context.Context) error
}

// OnModuleDestroy runs during app shutdown before OnApplicationShutdown.
type OnModuleDestroy interface {
	OnModuleDestroy(ctx context.Context) error
}

// OnApplicationShutdown runs at the end of app shutdown.
type OnApplicationShutdown interface {
	OnApplicationShutdown(ctx context.Context) error
}

type lifecycleError struct {
	Code     string
	Hook     string
	Module   string
	Provider string
	Err      error
}

func (e *lifecycleError) Error() string {
	return e.Code + ": " + e.Hook + " failed for " + e.Provider + " in module " + e.Module + ": " + e.Err.Error()
}

func (e *lifecycleError) Unwrap() error {
	return e.Err
}

func hookError(hook string, provider *providerState, err error) error {
	if err == nil {
		return nil
	}
	return &lifecycleError{
		Code:     "LIFECYCLE_HOOK_FAILED",
		Hook:     hook,
		Module:   provider.module.name,
		Provider: describeProvider(provider.provider),
		Err:      err,
	}
}

func callLifecycleHook(ctx context.Context, provider *providerState, hook string) error {
	switch hook {
	case "OnModuleInit":
		if target, ok := provider.instance.(OnModuleInit); ok {
			return hookError(hook, provider, target.OnModuleInit(ctx))
		}
	case "OnApplicationBootstrap":
		if target, ok := provider.instance.(OnApplicationBootstrap); ok {
			return hookError(hook, provider, target.OnApplicationBootstrap(ctx))
		}
	case "BeforeApplicationShutdown":
		if target, ok := provider.instance.(BeforeApplicationShutdown); ok {
			return hookError(hook, provider, target.BeforeApplicationShutdown(ctx))
		}
	case "OnModuleDestroy":
		if target, ok := provider.instance.(OnModuleDestroy); ok {
			return hookError(hook, provider, target.OnModuleDestroy(ctx))
		}
	case "OnApplicationShutdown":
		if target, ok := provider.instance.(OnApplicationShutdown); ok {
			return hookError(hook, provider, target.OnApplicationShutdown(ctx))
		}
	}
	return nil
}
