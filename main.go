package main

import (
	"go-download/internal/web"
	"log"
)

func main() {
	r := web.SetupRouter()
	port := ":11235"
	log.Printf("starting go-download server on %s...\n", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
