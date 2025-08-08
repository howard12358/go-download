package main

import (
	"go-download/internal/web"
	"log"
)

func main() {
	r := web.SetupRouter()
	addr := ":11235"
	log.Printf("starting go-download server on %s…\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
