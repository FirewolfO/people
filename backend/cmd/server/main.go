package main

import (
	"log"
	"time"

	"people/internal/api"
	"people/internal/config"
	permissionclient "people/internal/permission"
	"people/internal/security"
	"people/internal/service"
	"people/internal/store"
)

func main() {
	cfg := config.Load()
	database, err := store.Open(cfg.DatabaseDSN, cfg.PermissionClientID, cfg.PermissionClientSecret, cfg.PermissionRedirectURIs, store.OAuthClientSeed{
		ClientID: cfg.GatewayClientID, Name: "Gateway 管理系统", ClientSecret: cfg.GatewayClientSecret,
		RedirectURIs: cfg.GatewayRedirectURIs, AllowedScopes: []string{"openid", "profile"},
	}, store.OAuthClientSeed{
		ClientID: cfg.BlogClientID, Name: "内部博客", ClientSecret: cfg.BlogClientSecret,
		RedirectURIs: cfg.BlogRedirectURIs, AllowedScopes: []string{"openid", "profile"},
	}, store.OAuthClientSeed{
		ClientID: cfg.AIWorkbenchClientID, Name: "AI 工作台", ClientSecret: cfg.AIWorkbenchClientSecret,
		RedirectURIs: cfg.AIWorkbenchRedirectURIs, AllowedScopes: []string{"openid", "profile"},
	}, store.OAuthClientSeed{
		ClientID: cfg.AppCenterClientID, Name: "App Center", ClientSecret: cfg.AppCenterClientSecret,
		RedirectURIs: cfg.AppCenterRedirectURIs, AllowedScopes: []string{"openid", "profile"},
	}, store.OAuthClientSeed{
		ClientID: cfg.LinkUpClientID, Name: "语雀", ClientSecret: cfg.LinkUpClientSecret,
		RedirectURIs: cfg.LinkUpRedirectURIs, AllowedScopes: []string{"openid", "profile"},
	})
	if err != nil {
		log.Fatalf("open people database: %v", err)
	}
	defer database.Close()

	authorizer := permissionclient.New(cfg.PermissionAPIBaseURL, cfg.PermissionServiceID, cfg.PermissionServiceSecret)
	svc := service.New(database, cfg.SessionTTL, authorizer)
	verifier := security.NewGatewayVerifier(cfg.GatewayAccessKey, cfg.GatewaySecretKey, 5*time.Minute)
	innerVerifier := security.NewGatewayVerifier(cfg.InnerAccessKey, cfg.InnerSecretKey, 5*time.Minute)
	api.NewServer(cfg.Address, verifier, innerVerifier, svc, cfg.SessionTTL, cfg.CookieSecure).Spin()
}
