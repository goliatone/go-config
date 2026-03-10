package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type testApp struct {
	Name     string `koanf:"name"`
	Env      string `koanf:"env"`
	Version  string `koanf:"version"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"database"`
	Server struct {
		Env string `koanf:"env"`
	} `koanf:"server"`
}

func (a testApp) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("app name is required")
	}
	return nil
}

type invalidConfig struct {
	Field string `koanf:"field"`
}

func (c invalidConfig) Validate() error { return errors.New("invalid config") }

type selectAppConfig struct {
	App struct {
		Env string `koanf:"env"`
	} `koanf:"app"`
	Reminders struct {
		MaxReminders int `koanf:"max_reminders"`
	} `koanf:"reminders"`
}

func (c selectAppConfig) Validate() error {
	if strings.TrimSpace(c.App.Env) == "" {
		return fmt.Errorf("app.env is required")
	}
	if c.Reminders.MaxReminders <= 0 {
		return fmt.Errorf("reminders.max_reminders must be greater than zero")
	}
	return nil
}

func TestContainerLoadFromFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "config_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := `{"name": "TestApp", "env": "testing", "database": {"dsn": "test-dsn"}}`
	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	app := &testApp{}

	container := New(app).WithConfigPath(tmpfile.Name())

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if app.Name != "TestApp" {
		t.Errorf("expected App 'TestApp', got %q", app.Name)
	}

	if app.Env != "testing" {
		t.Errorf("expected Env 'testing', got %q", app.Env)
	}

	if app.Database.DSN != "test-dsn" {
		t.Errorf("expected Database.DSN 'test-dsn', got %q", app.Database.DSN)
	}
}

func TestEnvLoader(t *testing.T) {

	os.Setenv("APP_NAME", "nameValue")
	os.Setenv("APP_DATABASE__DSN", "dsnValue")
	defer os.Unsetenv("APP_NAME")
	defer os.Unsetenv("APP_DATABASE__DSN")

	cfg := &testApp{}

	loaderFactory := EnvProvider[*testApp]("APP_", "__")

	container := New(cfg).
		WithConfigPath(""). // we need to disable default config
		WithProvider(loaderFactory)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "nameValue" {
		t.Errorf("expected Name 'nameValue', got %q", cfg.Name)
	}

	if cfg.Database.DSN != "dsnValue" {
		t.Errorf("expected DSN 'dsnValue', got %q", cfg.Database.DSN)
	}
}

func TestFlagLoader(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("name", "default-name", "usage")
	fs.String("database.dsn", "default-dsn", "usage")
	fs.Parse([]string{"--name=nameFlag"})
	fs.Parse([]string{"--database.dsn=dsnFlag"})

	cfg := &testApp{}

	loaderFactory := FlagsProvider[*testApp](fs)
	container := New(cfg).
		WithConfigPath(""). // we need to disable default config
		WithProvider(loaderFactory)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "nameFlag" {
		t.Errorf("expected Name 'nameFlag', got %q", cfg.Name)
	}

	if cfg.Database.DSN != "dsnFlag" {
		t.Errorf("expected DSN 'dsnFlag', got %q", cfg.Database.DSN)
	}
}

func TestStructProvider(t *testing.T) {
	baseStruct := testApp{Name: "structValue"}
	cfg := &testApp{}

	loaderFactory := StructProvider[*testApp](baseStruct)
	container := New(cfg).
		WithConfigPath(""). // we need to disable default config
		WithProvider(loaderFactory)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Name != "structValue" {
		t.Errorf("expected Field 'structValue', got %q", cfg.Name)
	}
}

func TestContainerPreservesPointerIdentity(t *testing.T) {
	cfg := &testApp{}
	loaderFactory := StructProvider[*testApp](testApp{Name: "from-struct"})
	container := New(cfg).
		WithConfigPath("").
		WithProvider(loaderFactory)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if container.Raw() != cfg {
		t.Fatalf("expected container to retain original config pointer")
	}
	if cfg.Name != "from-struct" {
		t.Fatalf("expected struct to be populated via cfgx build")
	}
}

func TestOptionalProvider(t *testing.T) {
	dummyErr := errors.New("dummy error")
	dummyFactory := func(c *Container[testApp]) (Provider, error) {
		return &Loader{
			providerType: ProviderTypeDefault,
			order:        1,
			load: func(ctx context.Context, k *koanf.Koanf) error {
				return dummyErr
			},
		}, nil
	}

	optFactory := OptionalProvider(dummyFactory, func(err error) bool {
		return errors.Is(err, dummyErr)
	})

	loader, err := optFactory(nil)
	if err != nil {
		t.Fatalf("OptionalProvider creation failed: %v", err)
	}

	k := koanf.New(".")

	if err := loader.Load(context.Background(), k); err != nil {
		t.Errorf("Expected error to be ignored, got: %v", err)
	}
}

func TestContainerSelectSolver_DefaultChainResolves(t *testing.T) {
	cfg := &selectAppConfig{}
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "development",
		},
		"reminders": map[string]any{
			"$select":  "${app.env}",
			"$default": "production",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*selectAppConfig](defaultValues))

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Reminders.MaxReminders != 2 {
		t.Fatalf("expected reminders.max_reminders=2, got %d", cfg.Reminders.MaxReminders)
	}
}

func TestContainerSelectSolver_ErrorPropagatesWithMetadata(t *testing.T) {
	cfg := &selectAppConfig{}
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "staging",
		},
		"reminders": map[string]any{
			"$select": "app.env",
			"development": map[string]any{
				"max_reminders": 2,
			},
		},
	}

	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*selectAppConfig](defaultValues))

	err := container.Load(context.Background())
	if err == nil {
		t.Fatalf("expected load failure for unresolved select")
	}

	var cfgErr *errors.Error
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected go-errors wrapper, got %v", err)
	}
	if cfgErr.TextCode != "CONFIG_SELECT_RESOLUTION_FAILED" {
		t.Fatalf("expected CONFIG_SELECT_RESOLUTION_FAILED, got %q", cfgErr.TextCode)
	}
	if got := strings.TrimSpace(fmt.Sprint(cfgErr.Metadata["solver"])); got != "select" {
		t.Fatalf("expected solver metadata 'select', got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprint(cfgErr.Metadata["select_path"])); got != "app.env" {
		t.Fatalf("expected select_path metadata app.env, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprint(cfgErr.Metadata["select_value"])); got != "staging" {
		t.Fatalf("expected select_value metadata staging, got %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprint(cfgErr.Metadata["failing_node_path"])); got != "reminders" {
		t.Fatalf("expected failing_node_path reminders, got %q", got)
	}

	var selectErr *solvers.SelectResolutionError
	if !errors.As(err, &selectErr) {
		t.Fatalf("expected SelectResolutionError in chain, got %v", err)
	}
}

func TestContainerWithSolversReplacesDefaultsAndCanDisableSelect(t *testing.T) {
	cfg := &selectAppConfig{}
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "development",
		},
		"reminders": map[string]any{
			"$select":  "${app.env}",
			"$default": "production",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	container := New(cfg).
		WithConfigPath("").
		WithSolvers(
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
			solvers.NewExpressionSolver("{{", "}}"),
		).
		WithProvider(DefaultValuesProvider[*selectAppConfig](defaultValues))

	err := container.Load(context.Background())
	if err == nil {
		t.Fatalf("expected load failure when select solver is removed")
	}
	if !strings.Contains(err.Error(), "reminders.max_reminders must be greater than zero") {
		t.Fatalf("expected validation failure from unresolved reminders, got %v", err)
	}
}
