package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"people/internal/edge"
)

func main() {
	address := env("PEOPLE_EDGE_ADDR", ":8082")
	proxy, err := edge.New(
		env("PEOPLE_UPSTREAM_URL", "http://127.0.0.1:8085"),
		env("PEOPLE_GATEWAY_ACCESS_KEY", "gwak_gateway_local"),
		env("PEOPLE_GATEWAY_SECRET_KEY", "local-development-gateway-signin-secret-key"),
	)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Addr: address, Handler: proxy, ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 120 * time.Second,
	}
	log.Printf("People edge listening on %s", address)
	log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
