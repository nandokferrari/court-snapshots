package main

import (
	"log"

	"github.com/nandokferrari/court-snapshots/config"
	"github.com/nandokferrari/court-snapshots/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	srv := server.New(cfg)
	log.Printf("court-snapshots server starting on :%s", cfg.Port)
	log.Fatal(srv.ListenAndServe())
}
