package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/r6m/gest"
)

const (
	// MessageText is a text WebSocket message.
	MessageText = 1
	// MessageBinary is a binary WebSocket message.
	MessageBinary = 2
	// CloseNormalClosure is the standard normal closure code.
	CloseNormalClosure = 1000
	// CloseGoingAway is the standard going away closure code.
	CloseGoingAway = 1001
	// CloseUnsupportedData is the standard unsupported data closure code.
	CloseUnsupportedData = 1003
	// ClosePolicyViolation is the standard policy violation closure code.
	ClosePolicyViolation = 1008
	// CloseInternalError is the standard internal error closure code.
	CloseInternalError = 1011
)

// Options configures the optional WebSocket module.
type Options struct {
	Adapter   Adapter
	Codec     Codec
	Hooks     Hooks
	IDFactory IDFactory
	Gateways  []gest.Token
}

// Hooks contains connection lifecycle callbacks.
type Hooks struct {
	OnConnect    func(context.Context, *Client) error
	OnDisconnect func(context.Context, *Client, CloseError) error
}

// IDFactory creates stable client IDs.
type IDFactory func(*http.Request) string

// Module returns a Gest module that provides a WebSocket server through DI.
func Module(options Options, gateways ...gest.Provider) gest.Module {
	providers := []gest.Provider{
		gest.Provide(func() *Server {
			return NewServer(options)
		}),
		gest.Provide(func(server *Server) *Registrar {
			tokens := append([]gest.Token(nil), options.Gateways...)
			tokens = append(tokens, gatewayTokens(gateways)...)
			return NewRegistrar(server, tokens)
		}),
	}
	providers = append(providers, gateways...)
	return gest.NewModule(gest.ModuleConfig{
		Name:      "WebSocketModule",
		Providers: gest.Providers(providers...),
	})
}

// Gateway declares a WebSocket gateway provider.
func Gateway(constructor any, options ...gest.ProviderOption) gest.Provider {
	return gest.Provide(constructor, options...)
}

// Handler handles one decoded WebSocket message payload.
type Handler func(context.Context, *Client, json.RawMessage) error

// SubscriptionDefinition describes one generated WebSocket message subscription.
type SubscriptionDefinition struct {
	Event  string
	Handle Handler
}

// GatewayDefinition describes generated gateway metadata.
type GatewayDefinition struct {
	Name          string
	Path          string
	Subscriptions []SubscriptionDefinition
}

// DescribedGateway is implemented by generated gateway metadata.
type DescribedGateway interface {
	GestGateway() GatewayDefinition
}

// Handle adapts a typed gateway subscription method to generated metadata.
func Handle[T any](handler func(context.Context, *Client, T) error) Handler {
	return func(ctx context.Context, client *Client, payload json.RawMessage) error {
		var message T
		if err := json.Unmarshal(payload, &message); err != nil {
			return fmt.Errorf("WEBSOCKET_INVALID_PAYLOAD: decode %s: %w", typeNameOf[T](), err)
		}
		return handler(ctx, client, message)
	}
}

// Registrar registers generated gateway metadata as HTTP upgrade routes.
type Registrar struct {
	server *Server
	tokens []gest.Token
}

// NewRegistrar creates a gateway route registrar.
func NewRegistrar(server *Server, tokens []gest.Token) *Registrar {
	return &Registrar{server: server, tokens: append([]gest.Token(nil), tokens...)}
}

// RegisterRoutes registers one upgrade route for each generated gateway.
func (r *Registrar) RegisterRoutes(ctx gest.RouteRegistrationContext) error {
	if r == nil || r.server == nil {
		return fmt.Errorf("WEBSOCKET_INVALID_REGISTRAR: server is nil")
	}
	for _, token := range r.tokens {
		value, err := ctx.Container.Resolve(token)
		if err != nil {
			return err
		}
		gateway, ok := value.(DescribedGateway)
		if !ok {
			return fmt.Errorf("WEBSOCKET_INVALID_GATEWAY: provider %s does not implement websocket.DescribedGateway", token)
		}
		definition := gateway.GestGateway()
		if err := validateGatewayDefinition(definition); err != nil {
			return err
		}
		registeredGateway := definition
		if err := ctx.Register(gest.RouteRuntimeConfig{
			Method:  http.MethodGet,
			Path:    registeredGateway.Path,
			Handler: r.server.gatewayHandler(registeredGateway),
		}); err != nil {
			return err
		}
	}
	return nil
}

// Adapter is the net/http-compatible upgrade boundary.
type Adapter interface {
	Upgrade(response http.ResponseWriter, request *http.Request) (Connection, error)
}

// Connection is the minimal transport contract a Client needs.
type Connection interface {
	Read(ctx context.Context) (Message, error)
	Write(ctx context.Context, message Message) error
	Close(code int, reason string) error
}

// Message is one WebSocket message.
type Message struct {
	Type int
	Data []byte
}

// Codec encodes and decodes application payloads.
type Codec interface {
	Encode(value any) (Message, error)
	Decode(message Message, value any) error
}

// JSONCodec encodes values as text WebSocket messages containing JSON.
type JSONCodec struct{}

// Encode marshals value into a text message.
func (c JSONCodec) Encode(value any) (Message, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Message{}, err
	}
	return Message{Type: MessageText, Data: data}, nil
}

// Decode unmarshals a text or binary message into value.
func (c JSONCodec) Decode(message Message, value any) error {
	if message.Type != MessageText && message.Type != MessageBinary {
		return fmt.Errorf("WEBSOCKET_INVALID_MESSAGE: unsupported message type %d", message.Type)
	}
	return json.Unmarshal(message.Data, value)
}

// Server upgrades HTTP requests and creates clients.
type Server struct {
	adapter   Adapter
	codec     Codec
	hooks     Hooks
	idFactory IDFactory
	mu        sync.Mutex
	clients   map[string]*Client
	shutdown  bool
}

// NewServer creates a WebSocket server with default JSON codec and net/http adapter.
func NewServer(options Options) *Server {
	adapter := options.Adapter
	if adapter == nil {
		adapter = NewNetHTTPAdapter()
	}
	codec := options.Codec
	if codec == nil {
		codec = JSONCodec{}
	}
	idFactory := options.IDFactory
	if idFactory == nil {
		idFactory = defaultIDFactory
	}
	return &Server{
		adapter:   adapter,
		codec:     codec,
		hooks:     options.Hooks,
		idFactory: idFactory,
		clients:   make(map[string]*Client),
	}
}

// Upgrade upgrades a net/http request after any caller-owned middleware or guards run.
func (s *Server) Upgrade(ctx context.Context, response http.ResponseWriter, request *http.Request) (*Client, error) {
	if s == nil || s.adapter == nil {
		return nil, fmt.Errorf("WEBSOCKET_INVALID_SERVER: adapter is nil")
	}
	if s.isShutdown() {
		return nil, fmt.Errorf("WEBSOCKET_SERVER_SHUTDOWN: server is shutting down")
	}
	if ctx == nil {
		ctx = request.Context()
	}
	connection, err := s.adapter.Upgrade(response, request)
	if err != nil {
		return nil, err
	}
	client := NewClient(ClientOptions{
		ID:         s.idFactory(request),
		Connection: connection,
		Codec:      s.codec,
		Hooks:      s.hooks,
	})
	if s.hooks.OnConnect != nil {
		if err := s.hooks.OnConnect(ctx, client); err != nil {
			_ = client.Close(CloseGoingAway, "connect hook failed")
			return nil, err
		}
	}
	s.addClient(client)
	return client, nil
}

// BeforeApplicationShutdown closes active clients during app shutdown.
func (s *Server) BeforeApplicationShutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.shutdown = true
	clients := make([]*Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()
	for _, client := range clients {
		if err := client.Close(CloseGoingAway, "server shutdown"); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (s *Server) gatewayHandler(definition GatewayDefinition) gest.HandlerFunc {
	subscriptions := make(map[string]Handler, len(definition.Subscriptions))
	for _, subscription := range definition.Subscriptions {
		subscriptions[subscription.Event] = subscription.Handle
	}
	return func(ctx *gest.Context) error {
		client, err := s.Upgrade(ctx.RawRequest().Context(), ctx.RawResponse(), ctx.RawRequest())
		if err != nil {
			return err
		}
		defer s.removeClient(client)
		s.dispatch(ctx.RawRequest().Context(), client, subscriptions)
		return nil
	}
}

func (s *Server) dispatch(ctx context.Context, client *Client, subscriptions map[string]Handler) {
	for {
		message, err := client.connection.Read(ctx)
		if err != nil {
			var closeError CloseError
			switch {
			case errors.As(err, &closeError):
				client.markClosed(ctx, closeError)
			case ctx.Err() != nil:
				_ = client.Close(CloseGoingAway, "request canceled")
			default:
				_ = client.Close(CloseGoingAway, "connection closed")
			}
			return
		}
		var envelope envelope
		if err := json.Unmarshal(message.Data, &envelope); err != nil {
			_ = client.Close(CloseUnsupportedData, "invalid json")
			return
		}
		if envelope.Event == "" {
			_ = client.Close(ClosePolicyViolation, "missing event")
			return
		}
		handler, ok := subscriptions[envelope.Event]
		if !ok {
			_ = client.Close(ClosePolicyViolation, "unknown event")
			return
		}
		if err := handler(ctx, client, envelope.Data); err != nil {
			_ = client.Close(CloseInternalError, "handler error")
			return
		}
	}
}

func (s *Server) addClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID()] = client
}

func (s *Server) removeClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client.ID())
}

func (s *Server) isShutdown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdown
}

type envelope struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// ClientOptions configures a client wrapper.
type ClientOptions struct {
	ID         string
	Connection Connection
	Codec      Codec
	Hooks      Hooks
}

// Client wraps one WebSocket connection.
type Client struct {
	id         string
	connection Connection
	codec      Codec
	hooks      Hooks
	closed     atomic.Bool
	closeOnce  sync.Once
}

// NewClient creates a client for an accepted WebSocket connection.
func NewClient(options ClientOptions) *Client {
	codec := options.Codec
	if codec == nil {
		codec = JSONCodec{}
	}
	return &Client{
		id:         options.ID,
		connection: options.Connection,
		codec:      codec,
		hooks:      options.Hooks,
	}
}

// ID returns the stable client ID.
func (c *Client) ID() string {
	if c == nil {
		return ""
	}
	return c.id
}

// Closed reports whether the client has been closed.
func (c *Client) Closed() bool {
	if c == nil {
		return true
	}
	return c.closed.Load()
}

// SendJSON sends a JSON-encoded application message.
func (c *Client) SendJSON(ctx context.Context, value any) error {
	if c == nil || c.connection == nil {
		return fmt.Errorf("WEBSOCKET_INVALID_CLIENT: connection is nil")
	}
	if c.Closed() {
		return fmt.Errorf("WEBSOCKET_CLIENT_CLOSED: client %q is closed", c.id)
	}
	message, err := c.codec.Encode(value)
	if err != nil {
		return err
	}
	return c.connection.Write(ctx, message)
}

// ReceiveJSON reads one message and decodes it as JSON.
func (c *Client) ReceiveJSON(ctx context.Context, value any) error {
	if c == nil || c.connection == nil {
		return fmt.Errorf("WEBSOCKET_INVALID_CLIENT: connection is nil")
	}
	if c.Closed() {
		return fmt.Errorf("WEBSOCKET_CLIENT_CLOSED: client %q is closed", c.id)
	}
	message, err := c.connection.Read(ctx)
	if err != nil {
		var closeError CloseError
		if errors.As(err, &closeError) {
			c.markClosed(ctx, closeError)
		}
		return err
	}
	return c.codec.Decode(message, value)
}

// Close closes the underlying connection and emits the disconnect hook once.
func (c *Client) Close(code int, reason string) error {
	if c == nil {
		return nil
	}
	var err error
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.connection != nil {
			err = c.connection.Close(code, reason)
		}
		c.notifyDisconnect(context.Background(), CloseError{Code: code, Reason: reason})
	})
	return err
}

func (c *Client) markClosed(ctx context.Context, closeError CloseError) {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.notifyDisconnect(ctx, closeError)
	})
}

func (c *Client) notifyDisconnect(ctx context.Context, closeError CloseError) {
	if c.hooks.OnDisconnect != nil {
		_ = c.hooks.OnDisconnect(ctx, c, closeError)
	}
}

// CloseError describes a WebSocket close frame or transport close.
type CloseError struct {
	Code   int
	Reason string
	Err    error
}

func (e CloseError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Reason != "" {
		return e.Reason
	}
	return fmt.Sprintf("websocket closed with code %d", e.Code)
}

func (e CloseError) Unwrap() error {
	return e.Err
}

var nextClientID atomic.Uint64

func defaultIDFactory(*http.Request) string {
	id := nextClientID.Add(1)
	return fmt.Sprintf("ws-%d", id)
}

func typeNameOf[T any]() string {
	var zero *T
	return reflect.TypeOf(zero).Elem().String()
}

func gatewayTokens(providers []gest.Provider) []gest.Token {
	tokens := make([]gest.Token, 0, len(providers))
	for _, provider := range providers {
		token, ok := providerToken(provider)
		if ok {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func providerToken(provider gest.Provider) (gest.Token, bool) {
	if provider.Name != "" {
		return gest.Named(provider.Name), true
	}
	resultType := providerResultType(provider)
	if resultType == nil {
		return gest.Token{}, false
	}
	return gest.Token{Type: resultType}, true
}

func providerResultType(provider gest.Provider) reflect.Type {
	if provider.Value != nil {
		return reflect.TypeOf(provider.Value)
	}
	function := reflect.TypeOf(provider.Constructor)
	if function == nil || function.Kind() != reflect.Func || function.NumOut() == 0 {
		return nil
	}
	return function.Out(0)
}

func validateGatewayDefinition(definition GatewayDefinition) error {
	if definition.Name == "" {
		return fmt.Errorf("WEBSOCKET_INVALID_GATEWAY: gateway name is empty")
	}
	if definition.Path == "" {
		return fmt.Errorf("WEBSOCKET_INVALID_GATEWAY: gateway %s path is empty", definition.Name)
	}
	if definition.Path[0] != '/' {
		return fmt.Errorf("WEBSOCKET_INVALID_GATEWAY: gateway %s path must start with /", definition.Name)
	}
	seen := make(map[string]struct{}, len(definition.Subscriptions))
	for _, subscription := range definition.Subscriptions {
		if subscription.Event == "" {
			return fmt.Errorf("WEBSOCKET_INVALID_SUBSCRIPTION: gateway %s event is empty", definition.Name)
		}
		if subscription.Handle == nil {
			return fmt.Errorf("WEBSOCKET_INVALID_SUBSCRIPTION: gateway %s event %s handler is nil", definition.Name, subscription.Event)
		}
		if _, ok := seen[subscription.Event]; ok {
			return fmt.Errorf("WEBSOCKET_INVALID_SUBSCRIPTION: gateway %s duplicate event %s", definition.Name, subscription.Event)
		}
		seen[subscription.Event] = struct{}{}
	}
	return nil
}
