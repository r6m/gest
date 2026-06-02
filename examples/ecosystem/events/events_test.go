package events_test

import (
	"context"
	"testing"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/events"
)

func TestEventsExample(t *testing.T) {
	audit := &auditStore{}
	requested := gest.NewModule(gest.ModuleConfig{
		Name: "EventsExample",
		Imports: gest.Imports(
			events.Module(events.Options{Global: true}),
			ordersModule(audit),
		),
	})
	container, err := gest.NewContainer(requested)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	listener, err := container.Resolve(gest.TokenOf[*orderCreatedListener]())
	if err != nil {
		t.Fatalf("Resolve listener returned error: %v", err)
	}
	typedListener, ok := listener.(*orderCreatedListener)
	if !ok {
		t.Fatalf("listener = %T, want *orderCreatedListener", listener)
	}
	if err := typedListener.OnModuleInit(context.Background()); err != nil {
		t.Fatalf("OnModuleInit returned error: %v", err)
	}
	service, err := container.Resolve(gest.TokenOf[*orderService]())
	if err != nil {
		t.Fatalf("Resolve service returned error: %v", err)
	}
	typedService, ok := service.(*orderService)
	if !ok {
		t.Fatalf("service = %T, want *orderService", service)
	}
	if err := typedService.Create(context.Background(), "order-1"); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if audit.last != "order-1" {
		t.Fatalf("audit last = %q, want order-1", audit.last)
	}
}

func ordersModule(audit *auditStore) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "OrdersModule",
		Providers: gest.Providers(
			gest.Value(audit),
			gest.Provide(newOrderService),
			gest.Provide(newOrderCreatedListener),
		),
	})
}

type orderCreated struct {
	ID string
}

type orderService struct {
	bus *events.Bus
}

func newOrderService(bus *events.Bus) *orderService {
	return &orderService{bus: bus}
}

func (s *orderService) Create(ctx context.Context, id string) error {
	return s.bus.Emit(ctx, "order.created", orderCreated{ID: id})
}

type auditStore struct {
	last string
}

type orderCreatedListener struct {
	bus   *events.Bus
	audit *auditStore
}

func newOrderCreatedListener(bus *events.Bus, audit *auditStore) *orderCreatedListener {
	return &orderCreatedListener{bus: bus, audit: audit}
}

func (l *orderCreatedListener) OnModuleInit(ctx context.Context) error {
	_ = ctx
	return events.RegisterListener(l.bus, l)
}

func (l *orderCreatedListener) GestEventListener() events.ListenerDefinition {
	return events.ListenerDefinition{
		Name: "OrderCreatedListener",
		Listeners: []events.EventListener{
			{
				Event:  "order.created",
				Handle: events.Handle(l.Handle),
			},
		},
	}
}

func (l *orderCreatedListener) Handle(ctx context.Context, event orderCreated) error {
	_ = ctx
	l.audit.last = event.ID
	return nil
}
