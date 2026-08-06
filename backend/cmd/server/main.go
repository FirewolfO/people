package main

import (
	"log"
	"time"

	"people/internal/api"
	"people/internal/config"
	"people/internal/security"
	"people/internal/service"
	"people/internal/store"
)

func main() {
	cfg := config.Load()
	database, err := store.Open(cfg.DatabaseDSN, cfg.PermissionClientID, cfg.PermissionClientSecret, cfg.PermissionRedirectURIs)
	if err != nil {
		log.Fatalf("open people database: %v", err)
	}
	defer database.Close()

	svc := service.New(database, cfg.SessionTTL)
	verifier := security.NewGatewayVerifier(cfg.GatewayAccessKey, cfg.GatewaySecretKey, 5*time.Minute)
	api.NewServer(cfg.Address, verifier, svc, cfg.SessionTTL, cfg.CookieSecure).Spin()
}
