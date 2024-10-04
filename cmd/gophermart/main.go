package main

import (
	"context"
	"log"

	"github.com/beliaevke/go-musthave-diploma/internal/app"
	"github.com/beliaevke/go-musthave-diploma/internal/config"
	"github.com/beliaevke/go-musthave-diploma/internal/db/migrations"
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
