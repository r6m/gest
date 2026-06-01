package generator

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPhase2IntegrationScanParseGenerateWrite(t *testing.T) {
	root := newFixture(t, map[string]string{
		"go.mod": "module example.test/app\n\ngo 1.26.2\n\nrequire (\n\tgithub.com/go-chi/chi/v5 v5.3.0\n\tgithub.com/r6m/gest v0.0.0\n)\n\nreplace github.com/r6m/gest => " + filepath.ToSlash(projectRoot(t)) + "\n",
		"go.sum": "github.com/go-chi/chi/v5 v5.3.0 h1:halUjDxhshgXHMrao5bB8eNBXo/rnzwr8m5m36glehM=\n" +
			"github.com/go-chi/chi/v5 v5.3.0/go.mod h1:R+tYY2hNuVUUjxoPtqUdgBqevM9s9njzkTLutVsOCto=\n",
		"users/controller.go": `package users

import "github.com/r6m/gest"

// @Controller("/users")
// @Tag("Users")
type UserController struct{}

type FindUserRequest struct{}
type FindUserResponse struct{}

// @Get("/")
// @Status(200)
// @Summary("List users")
func (c *UserController) List(ctx *gest.Context) error {
	return ctx.JSON(200, []string{"ada"})
}

// @Get("/:id")
// @Status(200)
// @Status(404)
// @Summary("Find user")
// @Description("Returns a user by ID")
func (c *UserController) Find(ctx *gest.Context, req *FindUserRequest) (*FindUserResponse, error) {
	return nil, nil
}
`,
	})

	files := phase2Generate(t, root)
	if len(files) != 1 {
		t.Fatalf("files length = %d, want 1", len(files))
	}
	assertGeneratedPath(t, root, files[0], "users/users_gest.gen.go")
	assertNoInit(t, files[0].Content)
	assertNoHiddenRegistry(t, files[0].Content)
	assertGoldenFile(t, "phase2_users_gest.gen.go", files[0].Content)

	repeated := phase2Generate(t, root)
	if !reflect.DeepEqual(files, repeated) {
		t.Fatalf("generated files changed between repeated scan/parse/generate runs")
	}

	firstResults, diagnostics := WriteGeneratedFiles(files)
	if len(diagnostics) != 0 {
		t.Fatalf("first write diagnostics = %#v, want none", diagnostics)
	}
	if len(firstResults) != 1 || !firstResults[0].Written {
		t.Fatalf("first write results = %#v, want written file", firstResults)
	}

	secondResults, diagnostics := WriteGeneratedFiles(repeated)
	if len(diagnostics) != 0 {
		t.Fatalf("second write diagnostics = %#v, want none", diagnostics)
	}
	if len(secondResults) != 1 || secondResults[0].Written {
		t.Fatalf("second write results = %#v, want unchanged file", secondResults)
	}

	runGoTest(t, root)
}

func phase2Generate(t *testing.T, root string) []GeneratedFile {
	t.Helper()

	packages, err := ScanPackages(root, ScanOptions{})
	if err != nil {
		t.Fatalf("ScanPackages returned error: %v", err)
	}
	controllers, diagnostics, err := ParseControllerRoutes(packages)
	if err != nil {
		t.Fatalf("ParseControllerRoutes returned error: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
	files, err := GenerateMetadataFiles(controllers)
	if err != nil {
		t.Fatalf("GenerateMetadataFiles returned error: %v", err)
	}
	return files
}

func assertGoldenFile(t *testing.T, name string, got []byte) {
	t.Helper()

	want := readFile(t, filepath.Join("testdata", "golden", name))
	if !bytes.Equal(got, want) {
		t.Fatalf("generated content:\n%s\nwant golden:\n%s", got, want)
	}
}

func assertNoHiddenRegistry(t *testing.T, content []byte) {
	t.Helper()

	for _, pattern := range [][]byte{
		[]byte("RegisterController"),
		[]byte("RegisterRoute"),
		[]byte("gest.Register"),
		[]byte("registry"),
	} {
		if bytes.Contains(content, pattern) {
			t.Fatalf("generated content contains hidden registry pattern %q:\n%s", pattern, content)
		}
	}
}
