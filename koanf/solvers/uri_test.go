package solvers

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"go.beyondstorage.io/v5/types"
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

func TestKSolver_URLs_embedded_not_replaced(t *testing.T) {
	rawValue := "prefix @file://testdata/version.txt"
	defaultValues := map[string]any{
		"version": rawValue,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolver("@", "://")
	out := solver.Solve(k)

	assert.Equal(t, rawValue, out.Get("version"))
}

func TestKSolver_URLs_invalid_paths_not_replaced(t *testing.T) {
	defaultValues := map[string]any{
		"traversal": "@file://../secrets.txt",
		"absolute":  "@file:///etc/passwd",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolverWithFS("@", "://", fstest.MapFS{
		"secrets.txt": &fstest.MapFile{Data: []byte("secret")},
	})
	out := solver.Solve(k)

	assert.Equal(t, "@file://../secrets.txt", out.Get("traversal"))
	assert.Equal(t, "@file:///etc/passwd", out.Get("absolute"))
}

type mockStorager struct {
	reads       int
	contentPath map[string]string
	errPath     map[string]error
}

func (m *mockStorager) ReadWithContext(_ context.Context, path string, w io.Writer, _ ...types.Pair) (int64, error) {
	m.reads++
	if err, ok := m.errPath[path]; ok {
		return 0, err
	}
	content, ok := m.contentPath[path]
	if !ok {
		return 0, fs.ErrNotExist
	}
	n, err := io.WriteString(w, content)
	return int64(n), err
}

func TestKSolver_StorageProtocol_Resolves(t *testing.T) {
	defaultValues := map[string]any{
		"password": "@storage://mock://tenant/config#secrets/password.txt",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	store := &mockStorager{
		contentPath: map[string]string{
			"secrets/password.txt": "super-secret\n",
		},
	}
	solver := NewURISolverWithFSAndOptions("@", "://", os.DirFS("."))
	solverImpl := solver.(*uris)
	solverImpl.newStorager = func(conn string) (storageReader, error) {
		assert.Equal(t, "mock://tenant/config", conn)
		return store, nil
	}
	out := solver.Solve(k)

	assert.Equal(t, "super-secret", out.Get("password"))
	assert.Equal(t, 1, store.reads)
}

func TestKSolver_StorageProtocol_CachesStoragerAndValueByURI(t *testing.T) {
	defaultValues := map[string]any{
		"password_1": "@storage://mock://tenant/config#secrets/password.txt",
		"password_2": "@storage://mock://tenant/config#secrets/password.txt",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	newStoragerCalls := 0
	store := &mockStorager{
		contentPath: map[string]string{
			"secrets/password.txt": "super-secret\n",
		},
	}
	solver := NewURISolver("@", "://")
	solverImpl := solver.(*uris)
	solverImpl.newStorager = func(conn string) (storageReader, error) {
		newStoragerCalls++
		assert.Equal(t, "mock://tenant/config", conn)
		return store, nil
	}
	out := solver.Solve(k)

	assert.Equal(t, "super-secret", out.Get("password_1"))
	assert.Equal(t, "super-secret", out.Get("password_2"))
	assert.Equal(t, 1, newStoragerCalls)
	assert.Equal(t, 1, store.reads)
}

func TestKSolver_StorageProtocol_CachesStoragerByConnection(t *testing.T) {
	defaultValues := map[string]any{
		"p1": "@storage://mock://tenant/config#secrets/password1.txt",
		"p2": "@storage://mock://tenant/config#secrets/password2.txt",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	newStoragerCalls := 0
	store := &mockStorager{
		contentPath: map[string]string{
			"secrets/password1.txt": "secret-1\n",
			"secrets/password2.txt": "secret-2\n",
		},
	}
	solver := NewURISolver("@", "://")
	solverImpl := solver.(*uris)
	solverImpl.newStorager = func(conn string) (storageReader, error) {
		newStoragerCalls++
		assert.Equal(t, "mock://tenant/config", conn)
		return store, nil
	}
	out := solver.Solve(k)

	assert.Equal(t, "secret-1", out.Get("p1"))
	assert.Equal(t, "secret-2", out.Get("p2"))
	assert.Equal(t, 1, newStoragerCalls)
	assert.Equal(t, 2, store.reads)
}

func TestKSolver_StorageProtocol_DefaultErrorStrategyLeavesValue(t *testing.T) {
	rawValue := "@storage://mock://tenant/config#missing/path.txt"
	defaultValues := map[string]any{
		"password": rawValue,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolver("@", "://")
	solverImpl := solver.(*uris)
	solverImpl.newStorager = func(_ string) (storageReader, error) {
		return &mockStorager{
			contentPath: map[string]string{},
		}, nil
	}
	out := solver.Solve(k)

	assert.Equal(t, rawValue, out.Get("password"))
}

func TestKSolver_StorageProtocol_ErrorStrategyRemoveKey(t *testing.T) {
	rawValue := "@storage://mock://tenant/config#missing/path.txt"
	defaultValues := map[string]any{
		"password": rawValue,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewURISolverWithOptions("@", "://", WithURIOnErrorRemove())
	solverImpl := solver.(*uris)
	solverImpl.newStorager = func(_ string) (storageReader, error) {
		return &mockStorager{
			errPath: map[string]error{
				"missing/path.txt": errors.New("read failed"),
			},
		}, nil
	}
	out := solver.Solve(k)

	assert.False(t, out.Exists("password"))
}
