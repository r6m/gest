package events_test

import (
	"context"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/events"
)

func TestEventsModuleInjectsBusAndRegistersListenerThroughLifecycle(t *testing.T) {
	root := gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(
			events.Module(events.Options{Global: true}),
		),
		Providers: gest.Providers(
			gest.Provide(newRecordingListener),
			gest.Provide(newEmitter),
		),
	})

	container, err := gest.NewContainer(root)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	listener, err := container.Resolve(gest.TokenOf[*recordingListener]())
	if err != nil {
		t.Fatalf("Resolve listener returned error: %v", err)
	}
	typedListener, ok := listener.(*recordingListener)
	if !ok {
		t.Fatalf("listener = %T, want *recordingListener", listener)
	}
	if err := typedListener.OnModuleInit(context.Background()); err != nil {
		t.Fatalf("OnModuleInit returned error: %v", err)
	}
	resolvedEmitter, err := container.Resolve(gest.TokenOf[*emitter]())
	if err != nil {
		t.Fatalf("Resolve emitter returned error: %v", err)
	}
	typedEmitter, ok := resolvedEmitter.(*emitter)
	if !ok {
		t.Fatalf("emitter = %T, want *emitter", resolvedEmitter)
	}
	if err := typedEmitter.bus.Emit(context.Background(), "sample.created", sampleEvent{ID: "sample"}); err != nil {
		t.Fatalf("Emit returned error: %v", err)
	}
	if typedListener.seen != "sample" {
		t.Fatalf("listener saw %q, want sample", typedListener.seen)
	}
}

type sampleEvent struct {
	ID string
}

type recordingListener struct {
	bus  *events.Bus
	seen string
}

func newRecordingListener(bus *events.Bus) *recordingListener {
	return &recordingListener{bus: bus}
}

func (l *recordingListener) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return events.RegisterListener(l.bus, l)
}

func (l *recordingListener) GestEventListener() events.ListenerDefinition {
	return events.ListenerDefinition{
		Name: "RecordingListener",
		Listeners: []events.EventListener{
			{
				Event:  "sample.created",
				Handle: events.Handle(l.Handle),
			},
		},
	}
}

func (l *recordingListener) Handle(ctx context.Context, event sampleEvent) error {
	_ = ctx
	l.seen = event.ID
	return nil
}

type emitter struct {
	bus *events.Bus
}

func newEmitter(bus *events.Bus) *emitter {
	return &emitter{bus: bus}
}
