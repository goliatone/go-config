package solvers

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestKSolver_base64(t *testing.T) {
	defaultValues := map[string]any{
		"password": "@base64://I3B3MTI7UmFkZCRhLjI0Mw==",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolver("@", "://")
	out := solver.Solve(k)

	expected := "#pw12;Radd$a.243"

	assert.Equal(
		t,
		expected,
		out.Get("password"),
	)
}

func TestKSolver_URLs(t *testing.T) {
	notMatching := "@file://nothing"
	defaultValues := map[string]any{
		"version": "@file://testdata/version.txt",
		"context": map[string]any{
			"version": "${version}",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolver("@", "://")
	out := solver.Solve(k)

	b, err := fs.ReadFile(os.DirFS("."), "testdata/version.txt")
	if err != nil {
		t.Fatalf("Error reading version.txt file: %s", err)
	}

	version := string(b)
	version = strings.TrimRight(version, "\n")

	assert.Equal(
		t,
		string(version),
		out.Get("version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}

func TestKSolver_URLs_with_fs(t *testing.T) {
	notMatching := "@file://nothing"
	defaultValues := map[string]any{
		"version": "@file://testdata/version.txt",
		"context": map[string]any{
			"version": "${version}",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	version := "12345.00.54321\n"
	testFS := fstest.MapFS{
		"testdata/version.txt": &fstest.MapFile{
			Data: []byte(version),
		},
	}
	solver := NewURISolverWithFS("@", "://", testFS)
	out := solver.Solve(k)

	version = strings.TrimRight(version, "\n")

	assert.Equal(
		t,
		version,
		out.Get("version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}
