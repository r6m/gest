package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
)

// Options configures the optional WebSocket module.
type Options struct {
	Adapter   Adapter
	Codec     Codec
	Hooks     Hooks
	IDFactory IDFactory
}

// Hooks contains connection lifecycle callbacks.
type Hooks struct {
	OnConnect    func(context.Context, *Client) error
	OnDisconnect func(context.Context, *Client, CloseError) error
}

// IDFactory creates stable client IDs.
type IDFactory func(*http.Request) string

// Module returns a Gest module that provides a WebSocket server through DI.
func Module(options Options) gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "WebSocketModule",
		Providers: gest.Providers(
			gest.Provide(func() *Server {
				return NewServer(options)
			}),
		),
	})
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
	}
}

// Upgrade upgrades a net/http request after any caller-owned middleware or guards run.
func (s *Server) Upgrade(ctx context.Context, response http.ResponseWriter, request *http.Request) (*Client, error) {
	if s == nil || s.adapter == nil {
		return nil, fmt.Errorf("WEBSOCKET_INVALID_SERVER: adapter is nil")
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
	return client, nil
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
