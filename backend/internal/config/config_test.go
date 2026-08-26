package config

import "testing"

func TestDefaultOAuthRedirectURIsIncludeNetworkFrontends(t *testing.T) {
	t.Setenv("PEOPLE_PERMISSION_REDIRECT_URIS", "")
	t.Setenv("PEOPLE_GATEWAY_REDIRECT_URIS", "")
	t.Setenv("PEOPLE_BLOG_REDIRECT_URIS", "")
	t.Setenv("PEOPLE_AI_WORKBENCH_REDIRECT_URIS", "")

	configured := Load()
	assertContains(t, configured.PermissionRedirectURIs, "http://10.251.237.216:5174/oauth/callback")
	assertContains(t, configured.PermissionRedirectURIs, "http://10.251.237.216:5175/oauth/callback")
	assertContains(t, configured.PermissionRedirectURIs, "http://10.251.237.216:5178/oauth/callback")
	assertContains(t, configured.GatewayRedirectURIs, "http://10.251.237.216:5175/oauth/callback")
	assertContains(t, configured.BlogRedirectURIs, "http://10.251.237.216:5179/oauth/callback")
	assertContains(t, configured.AIWorkbenchRedirectURIs, "http://10.251.237.216:5181/oauth/callback")
	if configured.AIWorkbenchClientID != "ai-workbench-ui" {
		t.Fatalf("unexpected AI Workbench OAuth client: %q", configured.AIWorkbenchClientID)
	}
	assertContains(t, configured.LinkUpRedirectURIs, "https://im.lxvb.top/oauth/callback")
	if configured.LinkUpClientID != "linkup-im" {
		t.Fatalf("unexpected LinkUp OAuth client: %q", configured.LinkUpClientID)
	}
}

func assertContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("%q is not configured in %v", expected, values)
}
