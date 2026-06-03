package websocket_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"

	"github.com/r6m/gest"
	"github.com/r6m/gest/modules/websocket"
)

func TestWebSocketGatewayRegistersRouteResolvesGatewayAndDispatchesMessages(t *testing.T) {
	app := gest.New()
	middlewareRan := false
	app.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
		return func(ctx *gest.Context) error {
			middlewareRan = true
			return next(ctx)
		}
	}))
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(websocket.Module(
			websocket.Options{
				Adapter:  websocket.NewNetHTTPAdapter(websocket.WithCheckOrigin(func(*http.Request) bool { return true })),
				Gateways: []gest.Token{gest.TokenOf[*chatGateway]()},
			},
		)),
		Providers: gest.Providers(gest.Provide(newChatService), websocket.Gateway(newChatGateway)),
	}))
	server := httptest.NewServer(app)
	defer server.Close()

	connection, response := dialWebSocket(t, server.URL, "/ws/chat")
	defer closeResponseBody(t, response)
	defer closeWebSocket(t, connection)
	_ = response
	if !middlewareRan {
		t.Fatal("app middleware did not run before upgrade")
	}

	writeJSON(t, connection, map[string]any{
		"event": "message.send",
		"data":  map[string]string{"body": "hello"},
	})
	var ack map[string]string
	readJSON(t, connection, &ack)
	if ack["body"] != "sent:hello" {
		t.Fatalf("ack body = %q, want sent:hello", ack["body"])
	}
}

func TestWebSocketGatewayMiddlewareCanRejectBeforeUpgrade(t *testing.T) {
	app := gest.New()
	app.Use(gest.MiddlewareFunc(func(next gest.HandlerFunc) gest.HandlerFunc {
		return func(ctx *gest.Context) error {
			_ = next
			return gest.Forbidden("blocked before upgrade")
		}
	}))
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(websocket.Module(
			websocket.Options{
				Adapter:  websocket.NewNetHTTPAdapter(websocket.WithCheckOrigin(func(*http.Request) bool { return true })),
				Gateways: []gest.Token{gest.TokenOf[*chatGateway]()},
			},
		)),
		Providers: gest.Providers(gest.Provide(newChatService), websocket.Gateway(newChatGateway)),
	}))
	server := httptest.NewServer(app)
	defer server.Close()

	dialer := gorilla.Dialer{}
	connection, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/chat", nil)
	if connection != nil {
		closeWebSocket(t, connection)
	}
	defer closeResponseBody(t, response)
	if err == nil {
		t.Fatal("Dial returned nil error, want blocked handshake")
	}
	if responseStatus(response) != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", responseStatus(response), http.StatusForbidden)
	}
}

func TestWebSocketGatewayClosesPredictablyForBadMessages(t *testing.T) {
	tests := []struct {
		name      string
		write     func(*testing.T, *gorilla.Conn)
		wantCode  int
		wantText  string
		gatewayOK bool
	}{
		{
			name: "invalid json",
			write: func(t *testing.T, connection *gorilla.Conn) {
				t.Helper()
				if err := connection.WriteMessage(gorilla.TextMessage, []byte(`{`)); err != nil {
					t.Fatalf("WriteMessage returned error: %v", err)
				}
			},
			wantCode: websocket.CloseUnsupportedData,
			wantText: "invalid json",
		},
		{
			name: "unknown event",
			write: func(t *testing.T, connection *gorilla.Conn) {
				t.Helper()
				writeJSON(t, connection, map[string]any{"event": "missing", "data": map[string]string{}})
			},
			wantCode: websocket.ClosePolicyViolation,
			wantText: "unknown event",
		},
		{
			name: "handler error",
			write: func(t *testing.T, connection *gorilla.Conn) {
				t.Helper()
				writeJSON(t, connection, map[string]any{"event": "message.fail", "data": map[string]string{"body": "bad"}})
			},
			wantCode: websocket.CloseInternalError,
			wantText: "handler error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newWebSocketIntegrationApp()
			server := httptest.NewServer(app)
			defer server.Close()

			connection, response := dialWebSocket(t, server.URL, "/ws/chat")
			defer closeResponseBody(t, response)
			defer closeWebSocket(t, connection)
			test.write(t, connection)

			_, _, err := connection.ReadMessage()
			var closeError *gorilla.CloseError
			if !errors.As(err, &closeError) {
				t.Fatalf("ReadMessage error = %v, want close error", err)
			}
			if closeError.Code != test.wantCode {
				t.Fatalf("close code = %d, want %d", closeError.Code, test.wantCode)
			}
			if !strings.Contains(closeError.Text, test.wantText) {
				t.Fatalf("close text = %q, want %q", closeError.Text, test.wantText)
			}
		})
	}
}

func TestWebSocketGatewayClientCloseAndAppShutdownArePredictable(t *testing.T) {
	app := newWebSocketIntegrationApp()
	server := httptest.NewServer(app)
	defer server.Close()

	connection, response := dialWebSocket(t, server.URL, "/ws/chat")
	defer closeResponseBody(t, response)
	if err := connection.WriteMessage(
		gorilla.CloseMessage,
		gorilla.FormatCloseMessage(websocket.CloseNormalClosure, "done"),
	); err != nil {
		t.Fatalf("WriteMessage close returned error: %v", err)
	}
	closeWebSocket(t, connection)

	active, activeResponse := dialWebSocket(t, server.URL, "/ws/chat")
	defer closeResponseBody(t, activeResponse)
	defer closeWebSocket(t, active)
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	_, _, err := active.ReadMessage()
	var closeError *gorilla.CloseError
	if !errors.As(err, &closeError) {
		t.Fatalf("ReadMessage error = %v, want close error", err)
	}
	if closeError.Code != websocket.CloseGoingAway {
		t.Fatalf("close code = %d, want %d", closeError.Code, websocket.CloseGoingAway)
	}
	if !strings.Contains(closeError.Text, "server shutdown") {
		t.Fatalf("close text = %q, want server shutdown", closeError.Text)
	}
}

func newWebSocketIntegrationApp() *gest.App {
	app := gest.New()
	app.Import(gest.NewModule(gest.ModuleConfig{
		Name: "AppModule",
		Imports: gest.Imports(websocket.Module(
			websocket.Options{
				Adapter:  websocket.NewNetHTTPAdapter(websocket.WithCheckOrigin(func(*http.Request) bool { return true })),
				Gateways: []gest.Token{gest.TokenOf[*chatGateway]()},
			},
		)),
		Providers: gest.Providers(gest.Provide(newChatService), websocket.Gateway(newChatGateway)),
	}))
	return app
}

type chatService struct{}

func newChatService() *chatService {
	return &chatService{}
}

func (s *chatService) Send(body string) string {
	return "sent:" + body
}

type chatGateway struct {
	service *chatService
}

func newChatGateway(service *chatService) *chatGateway {
	return &chatGateway{service: service}
}

type sendMessage struct {
	Body string `json:"body"`
}

func (g *chatGateway) Send(ctx context.Context, client *websocket.Client, message sendMessage) error {
	_ = ctx
	return client.SendJSON(context.Background(), map[string]string{"body": g.service.Send(message.Body)})
}

func (g *chatGateway) Fail(ctx context.Context, client *websocket.Client, message sendMessage) error {
	_ = ctx
	_ = client
	_ = message
	return errors.New("handler failed")
}

func (g *chatGateway) GestGateway() websocket.GatewayDefinition {
	return websocket.GatewayDefinition{
		Name: "ChatGateway",
		Path: "/ws/chat",
		Subscriptions: []websocket.SubscriptionDefinition{
			{Event: "message.send", Handle: websocket.Handle[sendMessage](g.Send)},
			{Event: "message.fail", Handle: websocket.Handle[sendMessage](g.Fail)},
		},
	}
}

func dialWebSocket(t *testing.T, serverURL string, path string) (*gorilla.Conn, *http.Response) {
	t.Helper()
	dialer := gorilla.Dialer{}
	connection, response, err := dialer.Dial("ws"+strings.TrimPrefix(serverURL, "http")+path, nil)
	if err != nil {
		var body []byte
		if response != nil && response.Body != nil {
			body, _ = io.ReadAll(response.Body)
		}
		t.Fatalf("Dial returned error: %v status=%v body=%s", err, responseStatus(response), body)
	}
	return connection, response
}

func closeResponseBody(t *testing.T, response *http.Response) {
	t.Helper()
	if response == nil || response.Body == nil {
		return
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("response body close returned error: %v", err)
	}
}

func responseStatus(response *http.Response) int {
	if response == nil {
		return 0
	}
	return response.StatusCode
}

func writeJSON(t *testing.T, connection *gorilla.Conn, value any) {
	t.Helper()
	if err := connection.WriteJSON(value); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
}

func readJSON(t *testing.T, connection *gorilla.Conn, value any) {
	t.Helper()
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline returned error: %v", err)
	}
	if err := connection.ReadJSON(value); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
}

func closeWebSocket(t *testing.T, connection *gorilla.Conn) {
	t.Helper()
	if connection == nil {
		return
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
