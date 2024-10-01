package main

import (
	"context"
	"log"

	"musthave-diploma/internal/app"
	"musthave-diploma/internal/config"
	"musthave-diploma/internal/db/migrations"
)

func main() {

	cfg := config.NewConfig()
	ctx := context.Background()

	if err := migrations.Run(cfg, ctx); err != nil {
		log.Fatal(err)
	}

	if err := app.Run(cfg, ctx); err != nil {
		log.Fatal(err)
	}

}
