package config

import (
	"context"
	"fmt"
	"os"
	"testing"

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

	opt := func(c *Container[*testApp]) error {
		c.configPath = tmpfile.Name()
		// force the default file loader
		c.providers = nil
		return nil
	}

	container, err := New(app, opt)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

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
	opt := func(c *Container[*testApp]) error {
		c.providers = nil
		loaderFactory := EnvProvider[*testApp]("APP_", "__")
		loader, err := loaderFactory(c)
		if err != nil {
			return err
		}
		c.providers = append(c.providers, loader)
		return nil
	}
	container, err := New(cfg, opt)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

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
	opt := func(c *Container[*testApp]) error {
		c.providers = nil
		loaderFactory := FlagsProvider[*testApp](fs)
		loader, err := loaderFactory(c)
		if err != nil {
			return err
		}
		c.providers = append(c.providers, loader)
		return nil
	}
	container, err := New(cfg, opt)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

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
	opt := func(c *Container[*testApp]) error {
		c.providers = nil
		loaderFactory := StructProvider[*testApp](baseStruct)
		loader, err := loaderFactory(c)
		if err != nil {
			return err
		}
		c.providers = append(c.providers, loader)
		return nil
	}
	container, err := New(cfg, opt)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Name != "structValue" {
		t.Errorf("expected Field 'structValue', got %q", cfg.Name)
	}
}

func TestOptionalProvider(t *testing.T) {
	dummyErr := errors.New("dummy error")
	dummyFactory := func(c *Container[testApp]) (Loader, error) {
		return Loader{
			Type:  LoaderTypeDefault,
			Order: 1,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
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

func TestValidationError(t *testing.T) {
	cfg := &invalidConfig{}
	dummyProvider := Loader{
		Type:  LoaderTypeDefault,
		Order: 1,
		Load: func(ctx context.Context, k *koanf.Koanf) error {
			return nil
		},
	}
	opt := func(c *Container[*invalidConfig]) error {
		c.providers = []Loader{dummyProvider}
		return nil
	}
	container, err := New(cfg, opt)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	err = container.Load(context.Background())
	if err == nil {
		t.Fatalf("Expected validation error, got nil")
	}

	if !errors.IsValidation(err) {
		t.Errorf("Expected validation error, got %T: %v", err, err)
	}
}
