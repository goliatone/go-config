package main

import (
	"context"
	"fmt"

	"github.com/goliatone/go-config/config"
	"github.com/goliatone/go-print"
)

type App struct {
	Name     string   `koanf:"app"`
	Env      string   `koanf:"env"`
	Database Database `koanf:"database"`
	config   *config.Container[*App]
}

func (c App) Validate() error {
	return nil
}

type Database struct {
	DSN string `koanf:"dsn"`
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
	fmt.Println(print.MaybePrettyJSON(app.config.K.All()))
	fmt.Println(app.Name)
	fmt.Println(app.Env)
	fmt.Println(app.Database.DSN)
}
