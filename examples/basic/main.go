package main

import (
	"context"
	"fmt"

	"github.com/goliatone/go-config/config"
)

type App struct {
	Name     string `koanf:"name"`
	Env      string `koanf:"env"`
	Version  string `koanf:"version"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"database"`
	Server struct {
		Env string `koanf:"env"`
	} `koanf:"server"`
	config *config.Container[*App]
}

func (c App) Validate() error {
	if c.Env == "" || c.Name == "" || c.Version == "" {
		return fmt.Errorf("missing required app config values")
	}

	if c.Database.DSN == "" {
		return fmt.Errorf("missing required database config values")
	}

	if c.Server.Env == "" {
		return fmt.Errorf("missing required server config values")
	}

	return nil
}

func main() {
	app := &App{}
	config, err := config.New(app)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	if err := config.Load(ctx); err != nil {
		panic(err)
	}

	app.config = config

	fmt.Println("====== APP ======")
	fmt.Println(app.Name)
	fmt.Println(app.Env)
	fmt.Println(app.Database.DSN)
}
