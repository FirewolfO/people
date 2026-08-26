package edge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyRewritesAndSignsPeopleOpenRequest(t *testing.T) {
	const accessKey = "gwak_people_production"
	const secretKey = "people-production-gateway-secret"
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		payload := sha256.Sum256(body)
		payloadHash := hex.EncodeToString(payload[:])
		canonical := strings.Join([]string{
			request.Method,
			request.URL.EscapedPath(),
			"a=first&z=last",
			request.Header.Get("X-Gateway-Timestamp"),
			request.Header.Get("X-Gateway-Nonce"),
			payloadHash,
		}, "\n")
		mac := hmac.New(sha256.New, []byte(secretKey))
		_, _ = mac.Write([]byte(canonical))
		if request.URL.Path != "/api/v1/auth/login" || request.Header.Get("X-Gateway-Credential") != accessKey ||
			request.Header.Get("X-Gateway-Signature") != hex.EncodeToString(mac.Sum(nil)) || request.Header.Get("Cookie") != "PEOPLE_XSRF=value" {
			t.Fatalf("unexpected proxied request path=%s headers=%v", request.URL.Path, request.Header)
		}
		writer.Header().Add("Set-Cookie", "PEOPLE_SESSION=session; HttpOnly")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	proxy, err := New(upstream.URL, accessKey, secretKey)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/open/people/auth/login?z=last&a=first", strings.NewReader(`{"username":"admin"}`))
	request.Header.Set("Cookie", "PEOPLE_XSRF=value")
	response := httptest.NewRecorder()
	proxy.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Set-Cookie"), "PEOPLE_SESSION") {
		t.Fatalf("response status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}
