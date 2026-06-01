package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInvalidConfigReturnsNonZero(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"gest.yaml": "build:\n  trimpath: 123\n",
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"generate"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	assertOutputContains(t, stderr.String(), "parse", "gest.yaml", "cannot unmarshal")
}

func TestPhase4GeneratedSmallAppCanGenerateAndBuild(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n\nrequire (\n\tgithub.com/go-chi/chi/v5 v5.3.0\n\tgithub.com/r6m/gest v0.0.0\n)\n\nreplace github.com/r6m/gest => " + filepath.ToSlash(projectRoot(t)) + "\n",
		"go.sum": "github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=\n" +
			"github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=\n",
		"cmd/api/main.go": `package main

import (
	"github.com/r6m/gest"
	"example.test/app/internal/project/team"
)

func main() {
	app := gest.New()
	app.Import(team.Module(team.Options{}))
	_ = app
}
`,
	})

	command := New()
	command.WorkDir = root
	runPhase4Command(t, command, []string{"g", "module", "project/team", "--no-update-parent"}, "CREATE internal/project/team/team.module.go")
	runPhase4Command(t, command, []string{"g", "service", "project/team"}, "CREATE internal/project/team/team.service.go", "UPDATE internal/project/team/team.module.go")
	runPhase4Command(t, command, []string{"g", "controller", "project/team"}, "CREATE internal/project/team/team.controller.go", "UPDATE internal/project/team/team.module.go")
	runPhase4Command(t, command, []string{"generate"}, "scanned packages: 2", "generated: 1 updated: 0 skipped: 0")
	runPhase4Command(t, command, []string{"build", "--no-generate"}, "go build -trimpath -o bin/app ./cmd/api")

	assertFileExists(t, filepath.Join(root, "internal", "project", "team", "team_gest.gen.go"))
	assertFileExists(t, filepath.Join(root, "bin", "app"))

	module := readFile(t, filepath.Join(root, "internal", "project", "team", "team.module.go"))
	assertOutputContains(t, module,
		"gest.Provide(NewTeamService)",
		"gest.Controller(NewTeamController)",
	)
}

func runPhase4Command(t *testing.T, command *CLI, args []string, wants ...string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := command.Run(context.Background(), args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("command %s failed with code %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), code, stdout.String(), stderr.String())
	}
	for _, want := range wants {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("command %s output missing %q:\n%s", strings.Join(args, " "), want, stdout.String())
		}
	}
}

func TestPhase4GeneratedSmallAppBuildFailureIsVisible(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n",
		"cmd/api/main.go": `package main

func main() {
	missing()
}
`,
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"build", "--no-generate"}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	assertOutputContains(t, stdout.String(), "go build -trimpath -o bin/app ./cmd/api")
	if !strings.Contains(stderr.String(), "undefined: missing") {
		t.Fatalf("expected compiler error to be visible, got stderr:\n%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "app")); err == nil {
		t.Fatal("did not expect build output after compiler failure")
	}
}
