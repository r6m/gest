package gest_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWebSocketCheckpointBoundaries(t *testing.T) {
	root := projectRoot(t)
	coreFiles := []string{
		"app.go",
		"binding.go",
		"container.go",
		"context.go",
		"controller.go",
		"json.go",
		"module.go",
		"provider.go",
		"router.go",
		"stream.go",
	}
	for _, file := range coreFiles {
		content := readFile(t, filepath.Join(root, file))
		if strings.Contains(content, "github.com/r6m/gest/modules/websocket") {
			t.Fatalf("core runtime file %s imports modules/websocket", file)
		}
	}

	golden := readFile(t, filepath.Join(root, "internal", "generator", "testdata", "golden", "chat_websocket.gen.go"))
	for _, forbidden := range []string{
		"func init(",
		"Register",
		"registry",
		"ScanPackages",
		"scanner",
		"parser.Parse",
	} {
		if strings.Contains(golden, forbidden) {
			t.Fatalf("generated gateway metadata contains forbidden pattern %q:\n%s", forbidden, golden)
		}
	}
	if !strings.Contains(golden, `import "github.com/r6m/gest/modules/websocket"`) {
		t.Fatalf("generated gateway metadata does not use public websocket APIs:\n%s", golden)
	}
}
