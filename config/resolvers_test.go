package config

import (
	"context"
	"testing"
)

func TestContainerSolvers(t *testing.T) {
	app := &testApp{}

	container, err := New(app,
		WithConfigPath[*testApp]("testdata/resolvers.json"),
	)
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

	if app.Version != "app.version" {
		t.Errorf("expected Server.Env to equal Env, got %q", app.Version)
	}

	if app.Database.DSN != "test-dsn" {
		t.Errorf("expected Database.DSN 'test-dsn', got %q", app.Database.DSN)
	}

	if app.Server.Env != app.Env {
		t.Errorf("expected Server.Env to equal Env, got %q", app.Server.Env)
	}
}
