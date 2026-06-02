package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateResourceCreatesMinimalSlice(t *testing.T) {
	root := moduleFixture(t, map[string]string{
		"internal/projects/projects.module.go": parentModuleSource("projects", "projects"),
	})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "resource", "projects/members"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertOutputContains(t, stdout.String(),
		"CREATE internal/projects/members/members.module.go",
		"CREATE internal/projects/members/members.service.go",
		"CREATE internal/projects/members/members.controller.go",
		"CREATE internal/projects/members/members.dto.go",
		"CREATE internal/projects/members/members.service_test.go",
		"CREATE internal/projects/members/members.controller_test.go",
		"UPDATE internal/projects/projects.module.go",
	)
	module := readFile(t, filepath.Join(root, "internal", "projects", "members", "members.module.go"))
	controller := readFile(t, filepath.Join(root, "internal", "projects", "members", "members.controller.go"))
	service := readFile(t, filepath.Join(root, "internal", "projects", "members", "members.service.go"))
	dto := readFile(t, filepath.Join(root, "internal", "projects", "members", "members.dto.go"))
	parent := readFile(t, filepath.Join(root, "internal", "projects", "projects.module.go"))
	assertOutputContains(t, module,
		`Name: "projects.members"`,
		"gest.Provide(NewMembersService)",
		"gest.Controller(NewMembersController)",
	)
	assertOutputContains(t, controller,
		`// @Controller("/members")`,
		"func NewMembersController(service *MembersService) *MembersController",
		"func (c *MembersController) List(ctx *gest.Context) (*ListMembersResponse, error)",
	)
	assertOutputContains(t, service,
		"type Members struct",
		"func (s *MembersService) List() []Members",
	)
	assertOutputContains(t, dto,
		"type MembersResponse struct",
		"type ListMembersResponse struct",
	)
	assertOutputContains(t, parent,
		`"example.test/app/internal/projects/members"`,
		"members.Module(members.Options{}),",
	)
	assertOutputExcludes(t, strings.ToLower(controller+service+dto), "database")
	assertOutputExcludes(t, strings.ToLower(controller+service+dto), "auth")
	assertOutputExcludes(t, strings.ToLower(controller+service+dto), "cache")
}

func TestGenerateResourceNoTestSkipsTests(t *testing.T) {
	root := moduleFixture(t, nil)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := New()
	command.WorkDir = root
	code := command.Run(context.Background(), []string{"g", "resource", "projects/members", "--no-update-parent", "--no-test"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	assertFileMissing(t, filepath.Join(root, "internal", "projects", "members", "members.service_test.go"))
	assertFileMissing(t, filepath.Join(root, "internal", "projects", "members", "members.controller_test.go"))
	assertOutputExcludes(t, stdout.String(), "_test.go")
}

func TestGenerateNestedResourceCanGenerateTestAndBuild(t *testing.T) {
	root := fixtureWithFiles(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n\nrequire (\n\tgithub.com/go-chi/chi/v5 v5.3.0\n\tgithub.com/r6m/gest v0.0.0\n)\n\nreplace github.com/r6m/gest => " + filepath.ToSlash(projectRoot(t)) + "\n",
		"go.sum": "github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=\n" +
			"github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=\n",
		"gest.yaml": `project:
  name: generated-resource
entry: ./cmd/api
generate:
  root: .
build:
  output: bin/app
  generate: false
  test: false
`,
		"cmd/api/main.go": `package main

import (
	"github.com/r6m/gest"
	"example.test/app/internal/app"
)

func main() {
	server := gest.New()
	server.Import(app.Module(app.Options{}))
	_ = server
}
`,
		"internal/app/app.module.go": parentModuleSource("app", "app"),
	})
	command := New()
	command.WorkDir = root

	runPhase4Command(t, command, []string{"g", "resource", "projects/members"}, "CREATE internal/projects/members/members.module.go", "UPDATE internal/app/app.module.go")
	runPhase4Command(t, command, []string{"generate"}, "generated: 1 updated: 0 skipped: 0")
	runPhase4Command(t, command, []string{"build", "--test", "--no-generate"}, "go test ./...", "go build -trimpath -o bin/app ./cmd/api")

	assertFileExists(t, filepath.Join(root, "internal", "projects", "members", "members_gest.gen.go"))
	assertFileExists(t, filepath.Join(root, "bin", "app"))
}
