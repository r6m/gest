package events

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/r6m/gest"
)

// Options configures the optional events module.
type Options struct {
	Global bool
}

// Module returns a Gest module that provides an in-process synchronous event bus.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name:   "EventsModule",
		Global: options.Global,
		Providers: gest.Providers(
			gest.Provide(NewBus),
		),
	})
}

// Bus emits events synchronously to registered listeners.
type Bus struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
}

// NewBus creates an empty event bus.
func NewBus() *Bus {
	return &Bus{listeners: make(map[string][]Listener)}
}

// Listener handles one emitted event value.
type Listener func(context.Context, any) error

// EventListener describes generated listener metadata.
type EventListener struct {
	Event  string
	Handle Listener
}

// ListenerDefinition describes all events handled by a provider.
type ListenerDefinition struct {
	Name      string
	Listeners []EventListener
}

// DescribedListener is implemented by generated listener metadata.
type DescribedListener interface {
	GestEventListener() ListenerDefinition
}

// On registers a listener for an event name.
func (b *Bus) On(event string, listener Listener) error {
	if b == nil {
		return fmt.Errorf("EVENTS_INVALID_BUS: bus is nil")
	}
	if event == "" {
		return fmt.Errorf("EVENTS_INVALID_LISTENER: event name is empty")
	}
	if listener == nil {
		return fmt.Errorf("EVENTS_INVALID_LISTENER: listener for %q is nil", event)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[event] = append(b.listeners[event], listener)
	return nil
}

// Emit synchronously calls registered listeners for event in registration order.
func (b *Bus) Emit(ctx context.Context, event string, payload any) error {
	if b == nil {
		return fmt.Errorf("EVENTS_INVALID_BUS: bus is nil")
	}
	if event == "" {
		return fmt.Errorf("EVENTS_INVALID_EVENT: event name is empty")
	}
	b.mu.RLock()
	listeners := append([]Listener(nil), b.listeners[event]...)
	b.mu.RUnlock()
	for _, listener := range listeners {
		if err := listener(ctx, payload); err != nil {
			return fmt.Errorf("EVENTS_LISTENER_FAILED: %s: %w", event, err)
		}
	}
	return nil
}

// RegisterListener registers generated metadata from a listener provider.
func RegisterListener(bus *Bus, listener DescribedListener) error {
	if listener == nil {
		return fmt.Errorf("EVENTS_INVALID_LISTENER: listener is nil")
	}
	definition := listener.GestEventListener()
	if definition.Name == "" {
		return fmt.Errorf("EVENTS_INVALID_LISTENER: listener name is empty")
	}
	for _, entry := range definition.Listeners {
		if err := bus.On(entry.Event, entry.Handle); err != nil {
			return err
		}
	}
	return nil
}

// Handle adapts a typed listener method to bus listener metadata.
func Handle[T any](handler func(context.Context, T) error) Listener {
	return func(ctx context.Context, payload any) error {
		event, ok := payload.(T)
		if !ok {
			return fmt.Errorf("EVENTS_INVALID_PAYLOAD: got %s, want %s", typeName(payload), typeNameOf[T]())
		}
		return handler(ctx, event)
	}
}

func typeName(value any) string {
	if value == nil {
		return "<nil>"
	}
	return reflect.TypeOf(value).String()
}

func typeNameOf[T any]() string {
	var zero *T
	return reflect.TypeOf(zero).Elem().String()
}
