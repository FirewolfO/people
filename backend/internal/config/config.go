package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                 string
	DatabaseDSN             string
	AllowedOrigins          []string
	GatewayAccessKey        string
	GatewaySecretKey        string
	InnerAccessKey          string
	InnerSecretKey          string
	SessionTTL              time.Duration
	CookieSecure            bool
	PermissionClientID      string
	PermissionClientSecret  string
	PermissionRedirectURIs  []string
	GatewayClientID         string
	GatewayClientSecret     string
	GatewayRedirectURIs     []string
	BlogClientID            string
	BlogClientSecret        string
	BlogRedirectURIs        []string
	AIWorkbenchClientID     string
	AIWorkbenchClientSecret string
	AIWorkbenchRedirectURIs []string
	PermissionAPIBaseURL    string
	PermissionServiceID     string
	PermissionServiceSecret string
}

func Load() Config {
	return Config{
		Address:                 env("PEOPLE_ADDR", ":8085"),
		DatabaseDSN:             env("PEOPLE_DB_DSN", "people.db"),
		AllowedOrigins:          split(env("PEOPLE_ALLOWED_ORIGINS", "http://localhost:5177,http://127.0.0.1:5177")),
		GatewayAccessKey:        env("PEOPLE_GATEWAY_ACCESS_KEY", "gwak_gateway_local"),
		GatewaySecretKey:        env("PEOPLE_GATEWAY_SECRET_KEY", "local-development-gateway-signin-secret-key"),
		InnerAccessKey:          env("PEOPLE_INNER_ACCESS_KEY", "gwak_permission_local"),
		InnerSecretKey:          env("PEOPLE_INNER_SECRET_KEY", "local-development-permission-gateway-secret-key"),
		SessionTTL:              time.Duration(envInt("PEOPLE_SESSION_HOURS", 12)) * time.Hour,
		CookieSecure:            strings.EqualFold(env("PEOPLE_COOKIE_SECURE", "false"), "true"),
		PermissionClientID:      env("PEOPLE_PERMISSION_CLIENT_ID", "permission-ui"),
		PermissionClientSecret:  env("PEOPLE_PERMISSION_CLIENT_SECRET", "permission-local-client-secret-change-me"),
		PermissionRedirectURIs:  split(env("PEOPLE_PERMISSION_REDIRECT_URIS", "http://localhost:5173/oauth/callback,http://127.0.0.1:5173/oauth/callback,http://localhost:5174/oauth/callback,http://127.0.0.1:5174/oauth/callback,http://10.251.237.216:5174/oauth/callback,http://localhost:5175/oauth/callback,http://127.0.0.1:5175/oauth/callback,http://10.251.237.216:5175/oauth/callback,http://localhost:5178/oauth/callback,http://127.0.0.1:5178/oauth/callback,http://10.251.237.216:5178/oauth/callback")),
		GatewayClientID:         env("PEOPLE_GATEWAY_CLIENT_ID", "gateway-admin-ui"),
		GatewayClientSecret:     env("PEOPLE_GATEWAY_CLIENT_SECRET", "gateway-admin-local-client-secret-change-me"),
		GatewayRedirectURIs:     split(env("PEOPLE_GATEWAY_REDIRECT_URIS", "http://localhost:5175/oauth/callback,http://127.0.0.1:5175/oauth/callback,http://10.251.237.216:5175/oauth/callback")),
		BlogClientID:            env("PEOPLE_BLOG_CLIENT_ID", "blog-ui"),
		BlogClientSecret:        env("PEOPLE_BLOG_CLIENT_SECRET", "blog-local-client-secret-change-me"),
		BlogRedirectURIs:        split(env("PEOPLE_BLOG_REDIRECT_URIS", "http://localhost:5179/oauth/callback,http://127.0.0.1:5179/oauth/callback,http://10.251.237.216:5179/oauth/callback")),
		AIWorkbenchClientID:     env("PEOPLE_AI_WORKBENCH_CLIENT_ID", "ai-workbench-ui"),
		AIWorkbenchClientSecret: env("PEOPLE_AI_WORKBENCH_CLIENT_SECRET", "ai-workbench-local-client-secret-change-me"),
		AIWorkbenchRedirectURIs: split(env("PEOPLE_AI_WORKBENCH_REDIRECT_URIS", "http://localhost:5181/oauth/callback,http://127.0.0.1:5181/oauth/callback,http://10.251.237.216:5181/oauth/callback")),
		PermissionAPIBaseURL:    env("PEOPLE_PERMISSION_API_BASE_URL", "http://127.0.0.1:8081/api/v1"),
		PermissionServiceID:     env("PEOPLE_PERMISSION_SERVICE_CLIENT_ID", "people-service"),
		PermissionServiceSecret: env("PEOPLE_PERMISSION_SERVICE_CLIENT_SECRET", "local-development-people-permission-secret-key"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func split(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}
