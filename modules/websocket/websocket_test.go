package websocket

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gorilla "github.com/gorilla/websocket"
	"github.com/r6m/gest"
)

func TestModuleProvidesServer(t *testing.T) {
	module := Module(Options{})
	container, err := gest.NewContainer(module)
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	value, err := container.Resolve(gest.TokenOf[*Server]())
	if err != nil {
		t.Fatalf("Resolve Server returned error: %v", err)
	}
	if value == nil {
		t.Fatal("Resolve Server returned nil")
	}
}

func TestJSONCodecEncodesAndDecodesTextMessages(t *testing.T) {
	codec := JSONCodec{}

	message, err := codec.Encode(struct {
		ID string `json:"id"`
	}{ID: "msg-1"})
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if message.Type != MessageText {
		t.Fatalf("message type = %d, want %d", message.Type, MessageText)
	}
	if got := string(message.Data); got != `{"id":"msg-1"}` {
		t.Fatalf("message data = %q, want JSON object", got)
	}

	var decoded struct {
		ID string `json:"id"`
	}
	if err := codec.Decode(message, &decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if decoded.ID != "msg-1" {
		t.Fatalf("decoded ID = %q, want msg-1", decoded.ID)
	}
}

func TestJSONCodecRejectsUnsupportedMessageTypes(t *testing.T) {
	err := JSONCodec{}.Decode(Message{Type: 99, Data: []byte(`{}`)}, &struct{}{})
	if err == nil {
		t.Fatal("Decode returned nil error, want unsupported message type")
	}
	if !strings.Contains(err.Error(), "WEBSOCKET_INVALID_MESSAGE") {
		t.Fatalf("Decode error = %v, want WEBSOCKET_INVALID_MESSAGE", err)
	}
}

func TestClientSendsAndReceivesJSON(t *testing.T) {
	connection := &fakeConnection{
		readMessages: []Message{{Type: MessageText, Data: []byte(`{"body":"hello"}`)}},
	}
	client := NewClient(ClientOptions{
		ID:         "client-1",
		Connection: connection,
	})

	if err := client.SendJSON(context.Background(), map[string]string{"body": "ok"}); err != nil {
		t.Fatalf("SendJSON returned error: %v", err)
	}
	if len(connection.writes) != 1 {
		t.Fatalf("writes length = %d, want 1", len(connection.writes))
	}
	if got := string(connection.writes[0].Data); got != `{"body":"ok"}` {
		t.Fatalf("written data = %q, want encoded JSON", got)
	}

	var received struct {
		Body string `json:"body"`
	}
	if err := client.ReceiveJSON(context.Background(), &received); err != nil {
		t.Fatalf("ReceiveJSON returned error: %v", err)
	}
	if received.Body != "hello" {
		t.Fatalf("received body = %q, want hello", received.Body)
	}
}

func TestClientLifecycleHooksAndCloseHandling(t *testing.T) {
	connection := &fakeConnection{}
	disconnects := 0
	client := NewClient(ClientOptions{
		ID:         "client-1",
		Connection: connection,
		Hooks: Hooks{
			OnDisconnect: func(ctx context.Context, client *Client, closeError CloseError) error {
				disconnects++
				if client.ID() != "client-1" {
					t.Fatalf("client ID = %q, want client-1", client.ID())
				}
				if closeError.Code != CloseNormalClosure {
					t.Fatalf("close code = %d, want %d", closeError.Code, CloseNormalClosure)
				}
				return nil
			},
		},
	})

	if err := client.Close(CloseNormalClosure, "done"); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !client.Closed() {
		t.Fatal("Closed = false, want true")
	}
	if connection.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", connection.closeCalls)
	}
	if disconnects != 1 {
		t.Fatalf("disconnects = %d, want 1", disconnects)
	}
	if err := client.Close(CloseNormalClosure, "again"); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	if connection.closeCalls != 1 || disconnects != 1 {
		t.Fatalf("closeCalls=%d disconnects=%d, want idempotent close", connection.closeCalls, disconnects)
	}
}

func TestClientReceiveCloseErrorMarksClosedAndNotifies(t *testing.T) {
	connection := &fakeConnection{readErr: CloseError{Code: CloseGoingAway, Reason: "bye"}}
	var gotClose CloseError
	client := NewClient(ClientOptions{
		ID:         "client-1",
		Connection: connection,
		Hooks: Hooks{
			OnDisconnect: func(ctx context.Context, client *Client, closeError CloseError) error {
				gotClose = closeError
				return nil
			},
		},
	})

	var payload struct{}
	err := client.ReceiveJSON(context.Background(), &payload)
	if err == nil {
		t.Fatal("ReceiveJSON returned nil error, want close error")
	}
	if !client.Closed() {
		t.Fatal("Closed = false, want true")
	}
	if gotClose.Code != CloseGoingAway || gotClose.Reason != "bye" {
		t.Fatalf("disconnect close = %#v, want going away bye", gotClose)
	}
}

func TestServerUpgradeUsesAdapterAndConnectHook(t *testing.T) {
	adapter := &fakeAdapter{connection: &fakeConnection{}}
	connected := false
	server := NewServer(Options{
		Adapter: adapter,
		IDFactory: func(request *http.Request) string {
			return "request-client"
		},
		Hooks: Hooks{
			OnConnect: func(ctx context.Context, client *Client) error {
				connected = true
				if client.ID() != "request-client" {
					t.Fatalf("client ID = %q, want request-client", client.ID())
				}
				return nil
			},
		},
	})

	client, err := server.Upgrade(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ws", nil))
	if err != nil {
		t.Fatalf("Upgrade returned error: %v", err)
	}
	if client.ID() != "request-client" {
		t.Fatalf("client ID = %q, want request-client", client.ID())
	}
	if adapter.calls != 1 {
		t.Fatalf("adapter calls = %d, want 1", adapter.calls)
	}
	if !connected {
		t.Fatal("OnConnect was not called")
	}
}

func TestServerUpgradeClosesConnectionWhenConnectHookFails(t *testing.T) {
	connection := &fakeConnection{}
	want := errors.New("blocked")
	server := NewServer(Options{
		Adapter: &fakeAdapter{connection: connection},
		Hooks: Hooks{
			OnConnect: func(ctx context.Context, client *Client) error {
				return want
			},
		},
	})

	client, err := server.Upgrade(context.Background(), httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ws", nil))
	if !errors.Is(err, want) {
		t.Fatalf("Upgrade error = %v, want %v", err, want)
	}
	if client != nil {
		t.Fatalf("client = %#v, want nil", client)
	}
	if connection.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", connection.closeCalls)
	}
}

func TestNetHTTPAdapterUpgradesAndExchangesMessages(t *testing.T) {
	server := NewServer(Options{
		Adapter: NewNetHTTPAdapter(WithCheckOrigin(func(*http.Request) bool { return true })),
	})
	received := make(chan string, 1)
	httpServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		client, err := server.Upgrade(request.Context(), response, request)
		if err != nil {
			t.Errorf("Upgrade returned error: %v", err)
			return
		}
		var payload struct {
			Body string `json:"body"`
		}
		if err := client.ReceiveJSON(request.Context(), &payload); err != nil {
			t.Errorf("ReceiveJSON returned error: %v", err)
			return
		}
		received <- payload.Body
		if err := client.SendJSON(request.Context(), map[string]string{"body": "ack"}); err != nil {
			t.Errorf("SendJSON returned error: %v", err)
		}
	}))
	defer httpServer.Close()

	dialer := gorilla.Dialer{}
	connection, dialResponse, err := dialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	if dialResponse != nil && dialResponse.Body != nil {
		defer func() {
			if err := dialResponse.Body.Close(); err != nil {
				t.Fatalf("dial response close returned error: %v", err)
			}
		}()
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}()

	if err := connection.WriteJSON(map[string]string{"body": "hello"}); err != nil {
		t.Fatalf("WriteJSON returned error: %v", err)
	}
	var response map[string]string
	if err := connection.ReadJSON(&response); err != nil {
		t.Fatalf("ReadJSON returned error: %v", err)
	}
	if response["body"] != "ack" {
		t.Fatalf("response body = %q, want ack", response["body"])
	}
	if got := <-received; got != "hello" {
		t.Fatalf("received = %q, want hello", got)
	}
}

type fakeAdapter struct {
	connection Connection
	calls      int
	err        error
}

func (a *fakeAdapter) Upgrade(http.ResponseWriter, *http.Request) (Connection, error) {
	a.calls++
	if a.err != nil {
		return nil, a.err
	}
	return a.connection, nil
}

type fakeConnection struct {
	readMessages []Message
	readErr      error
	writes       []Message
	closeCalls   int
	closeCode    int
	closeReason  string
}

func (c *fakeConnection) Read(ctx context.Context) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	if c.readErr != nil {
		return Message{}, c.readErr
	}
	if len(c.readMessages) == 0 {
		return Message{}, errors.New("no message")
	}
	message := c.readMessages[0]
	c.readMessages = c.readMessages[1:]
	return message, nil
}

func (c *fakeConnection) Write(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.writes = append(c.writes, message)
	return nil
}

func (c *fakeConnection) Close(code int, reason string) error {
	c.closeCalls++
	c.closeCode = code
	c.closeReason = reason
	return nil
}
