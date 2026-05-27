package main

import (
	"log"
	"net/http"

	"coast-monitoring/internal/config"
	httpx "coast-monitoring/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpx.NewRouter(httpx.Dependencies{}),
	}

	log.Printf("listening on %s", cfg.HTTPAddr)
	log.Fatal(server.ListenAndServe())
}
