package store

import (
	"path/filepath"
	"strings"
	"testing"

	"people/internal/model"
)

func TestOpenSeedsAdditionalOAuthClients(t *testing.T) {
	database, err := Open(
		filepath.Join(t.TempDir(), "people.db"),
		"permission-ui",
		"permission-secret",
		[]string{"http://permission.example/oauth/callback"},
		OAuthClientSeed{
			ClientID: "gateway-admin-ui", Name: "Gateway 管理系统", ClientSecret: "gateway-secret",
			RedirectURIs: []string{"http://gateway.example/oauth/callback"}, AllowedScopes: []string{"openid", "profile"},
		},
	)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	var clients []model.OAuthClient
	if err := database.DB.Order("client_id").Find(&clients).Error; err != nil {
		t.Fatalf("list OAuth clients: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("OAuth client count = %d, want 2", len(clients))
	}
	var gateway model.OAuthClient
	if err := database.DB.Where("client_id = ?", "gateway-admin-ui").First(&gateway).Error; err != nil {
		t.Fatalf("find Gateway OAuth client: %v", err)
	}
	if gateway.Name != "Gateway 管理系统" || gateway.RedirectURIs != "http://gateway.example/oauth/callback" || gateway.AllowedScopes != "openid profile" {
		t.Fatalf("Gateway OAuth client = %+v", gateway)
	}
	if gateway.SecretHash == "gateway-secret" || strings.TrimSpace(gateway.SecretHash) == "" {
		t.Fatal("Gateway OAuth client secret was not hashed")
	}
}
