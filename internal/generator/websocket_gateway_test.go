package generator

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGatewaysParsesGatewayAndSubscriptions(t *testing.T) {
	root := gatewayFixture(t, `package chat

import (
	"context"

	"github.com/r6m/gest/modules/websocket"
)

type SendMessage struct {
	Body string
}

type JoinMessage struct {
	Room string
}

// @Gateway("/ws/chat")
type ChatGateway struct{}

// @Subscribe("message.send")
func (g *ChatGateway) Send(ctx context.Context, client *websocket.Client, msg SendMessage) error {
	return nil
}

// @Subscribe("room.join")
func (g *ChatGateway) Join(ctx context.Context, client *websocket.Client, msg JoinMessage) error {
	return nil
}
`)

	gateways, diagnostics, err := ParseGateways(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseGateways returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	if len(gateways) != 1 {
		t.Fatalf("gateways length = %d, want 1", len(gateways))
	}
	gateway := gateways[0]
	if gateway.TypeName != "ChatGateway" || gateway.Path != "/ws/chat" {
		t.Fatalf("gateway = %#v, want ChatGateway /ws/chat", gateway)
	}
	gotSubscriptions := []Subscription{
		{Event: gateway.Subscriptions[0].Event, HandlerName: gateway.Subscriptions[0].HandlerName, MessageType: gateway.Subscriptions[0].MessageType},
		{Event: gateway.Subscriptions[1].Event, HandlerName: gateway.Subscriptions[1].HandlerName, MessageType: gateway.Subscriptions[1].MessageType},
	}
	wantSubscriptions := []Subscription{
		{Event: "message.send", HandlerName: "Send", MessageType: "SendMessage"},
		{Event: "room.join", HandlerName: "Join", MessageType: "JoinMessage"},
	}
	if !reflect.DeepEqual(gotSubscriptions, wantSubscriptions) {
		t.Fatalf("subscriptions = %#v, want %#v", gotSubscriptions, wantSubscriptions)
	}
}

func TestParseGatewaysRejectsInvalidGatewaySyntaxAndTarget(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"chat/gateway.go": `package chat

// @Gateway("ws/chat")
type MissingSlashGateway struct{}

// @Gateway("/ws/chat")
type (
	MultiA struct{}
	MultiB struct{}
)
`,
	})

	_, diagnostics, err := ParseGateways(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseGateways returned error: %v", err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics length = %d, want 2: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != DiagnosticInvalidDecoratorSyntax {
		t.Fatalf("first diagnostic = %#v, want invalid syntax", diagnostics[0])
	}
	if diagnostics[1].Code != DiagnosticInvalidTarget {
		t.Fatalf("second diagnostic = %#v, want invalid target", diagnostics[1])
	}
}

func TestParseGatewaysRejectsInvalidSubscribeTargetAndSignature(t *testing.T) {
	root := gatewayFixture(t, `package chat

import (
	"context"

	"github.com/r6m/gest/modules/websocket"
)

type SendMessage struct{}

// @Gateway("/ws/chat")
type ChatGateway struct{}

// @Subscribe("message.send")
func (g *ChatGateway) Send(ctx context.Context, msg SendMessage) error {
	return nil
}

type NotGateway struct{}

// @Subscribe("message.other")
func (g *NotGateway) Other(ctx context.Context, client *websocket.Client, msg SendMessage) error {
	return nil
}
`)

	_, diagnostics, err := ParseGateways(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseGateways returned error: %v", err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics length = %d, want 2: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Code != DiagnosticInvalidHandlerSignature {
		t.Fatalf("first diagnostic = %#v, want invalid signature", diagnostics[0])
	}
	if diagnostics[1].Code != DiagnosticInvalidTarget {
		t.Fatalf("second diagnostic = %#v, want invalid target", diagnostics[1])
	}
}

func TestGenerateGatewayMetadataGoldenAndDeterministic(t *testing.T) {
	root := gatewayFixture(t, validGatewaySource())
	gateways, diagnostics, err := ParseGateways(scanFixturePackages(t, root))
	if err != nil {
		t.Fatalf("ParseGateways returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}

	first, err := GenerateGatewayMetadataFiles(gateways)
	if err != nil {
		t.Fatalf("GenerateGatewayMetadataFiles returned error: %v", err)
	}
	second, err := GenerateGatewayMetadataFiles(gateways)
	if err != nil {
		t.Fatalf("second GenerateGatewayMetadataFiles returned error: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("generated files changed between repeated generation")
	}
	if len(first) != 1 {
		t.Fatalf("files length = %d, want 1", len(first))
	}
	assertGeneratedPath(t, root, first[0], "chat/chat_websocket.gen.go")
	assertNoInit(t, first[0].Content)
	assertNoHiddenRegistry(t, first[0].Content)
	for _, forbidden := range []string{"scanner", "ScanPackages", "init()", "Register"} {
		if strings.Contains(string(first[0].Content), forbidden) {
			t.Fatalf("generated content contains forbidden pattern %q:\n%s", forbidden, first[0].Content)
		}
	}
	assertGoldenFile(t, "chat_websocket.gen.go", first[0].Content)
}

func TestGenerateGatewayMetadataGeneratedFileCompiles(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n\nrequire (\n\tgithub.com/go-chi/chi/v5 v5.3.0\n\tgithub.com/gorilla/websocket v1.5.3\n\tgithub.com/r6m/gest v0.0.0\n)\n\nreplace github.com/r6m/gest => " + filepath.ToSlash(projectRoot(t)) + "\n",
		"go.sum": "github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=\n" +
			"github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=\n" +
			"github.com/gorilla/websocket v1.5.3 h1:saDtZ6Pbx/0u+bgYQ3q96pZgCzfhKXGPqt7kZ72aNNg=\n" +
			"github.com/gorilla/websocket v1.5.3/go.mod h1:YR8l580nyteQvAITg2hZ9XVh4b55+EU/adAjf1fMHhE=\n",
		"chat/gateway.go": validGatewaySource(),
	})
	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	gateways, diagnostics, err := ParseGateways(packages)
	if err != nil {
		t.Fatalf("ParseGateways returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	files, err := GenerateGatewayMetadataFiles(gateways)
	if err != nil {
		t.Fatalf("GenerateGatewayMetadataFiles returned error: %v", err)
	}
	results, diagnostics := WriteGeneratedFiles(files)
	if len(diagnostics) != 0 {
		t.Fatalf("write diagnostics = %#v, want none", diagnostics)
	}
	if len(results) != 1 || !results[0].Written {
		t.Fatalf("write results = %#v, want one written file", results)
	}
	runGoTest(t, root)
}

func validGatewaySource() string {
	return `package chat

import (
	"context"

	"github.com/r6m/gest/modules/websocket"
)

type SendMessage struct {
	Body string
}

type JoinMessage struct {
	Room string
}

// @Gateway("/ws/chat")
type ChatGateway struct{}

// @Subscribe("message.send")
func (g *ChatGateway) Send(ctx context.Context, client *websocket.Client, msg SendMessage) error {
	return nil
}

// @Subscribe("room.join")
func (g *ChatGateway) Join(ctx context.Context, client *websocket.Client, msg JoinMessage) error {
	return nil
}
`
}

func gatewayFixture(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.test/app\n\ngo 1.26.2\n")
	writeTestFile(t, root, "chat/gateway.go", source)
	return root
}
