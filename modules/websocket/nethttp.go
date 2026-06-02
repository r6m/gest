package websocket

import (
	"context"
	"errors"
	"net/http"
	"time"

	gorilla "github.com/gorilla/websocket"
)

// NetHTTPAdapter upgrades net/http requests using gorilla/websocket.
type NetHTTPAdapter struct {
	upgrader gorilla.Upgrader
}

// NetHTTPOption configures the net/http adapter.
type NetHTTPOption func(*NetHTTPAdapter)

// WithCheckOrigin configures the adapter origin check.
func WithCheckOrigin(check func(*http.Request) bool) NetHTTPOption {
	return func(adapter *NetHTTPAdapter) {
		adapter.upgrader.CheckOrigin = check
	}
}

// NewNetHTTPAdapter creates a net/http-compatible WebSocket adapter.
func NewNetHTTPAdapter(options ...NetHTTPOption) *NetHTTPAdapter {
	adapter := &NetHTTPAdapter{
		upgrader: gorilla.Upgrader{},
	}
	for _, option := range options {
		option(adapter)
	}
	return adapter
}

// Upgrade upgrades an HTTP request to a WebSocket connection.
func (a *NetHTTPAdapter) Upgrade(response http.ResponseWriter, request *http.Request) (Connection, error) {
	if a == nil {
		return nil, errors.New("WEBSOCKET_INVALID_ADAPTER: adapter is nil")
	}
	connection, err := a.upgrader.Upgrade(response, request, nil)
	if err != nil {
		return nil, err
	}
	return &netHTTPConnection{connection: connection}, nil
}

type netHTTPConnection struct {
	connection *gorilla.Conn
}

func (c *netHTTPConnection) Read(ctx context.Context) (Message, error) {
	if c == nil || c.connection == nil {
		return Message{}, errors.New("WEBSOCKET_INVALID_CONNECTION: connection is nil")
	}
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	messageType, data, err := c.connection.ReadMessage()
	if err != nil {
		if closeError, ok := err.(*gorilla.CloseError); ok {
			return Message{}, CloseError{Code: closeError.Code, Reason: closeError.Text, Err: err}
		}
		return Message{}, err
	}
	return Message{Type: messageType, Data: data}, nil
}

func (c *netHTTPConnection) Write(ctx context.Context, message Message) error {
	if c == nil || c.connection == nil {
		return errors.New("WEBSOCKET_INVALID_CONNECTION: connection is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.connection.WriteMessage(message.Type, message.Data)
}

func (c *netHTTPConnection) Close(code int, reason string) error {
	if c == nil || c.connection == nil {
		return nil
	}
	message := gorilla.FormatCloseMessage(code, reason)
	_ = c.connection.WriteControl(gorilla.CloseMessage, message, defaultCloseDeadline())
	return c.connection.Close()
}

func defaultCloseDeadline() time.Time {
	return time.Now().Add(time.Second)
}
