package permission

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func New(baseURL, clientID, clientSecret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), clientID: strings.TrimSpace(clientID), clientSecret: clientSecret,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (client *Client) Authorize(ctx context.Context, username string, codes []string) (map[string]bool, error) {
	body, err := json.Marshal(map[string]any{
		"principal":   map[string]string{"type": "user", "identifier": strings.TrimSpace(username)},
		"permissions": codes,
	})
	if err != nil {
		return nil, err
	}
	target := client.baseURL + "/openapi/authorize"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, err
	}
	nonce := hex.EncodeToString(nonceBytes)
	payloadSum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(payloadSum[:])
	canonical := strings.Join([]string{http.MethodPost, request.URL.EscapedPath(), request.URL.RawQuery, timestamp, nonce, payloadHash}, "\n")
	mac := hmac.New(sha256.New, []byte(client.clientSecret))
	_, _ = mac.Write([]byte(canonical))
	signature := hex.EncodeToString(mac.Sum(nil))
	request.Header.Set("Authorization", "Permission-HMAC-SHA256 Credential="+client.clientID+",Signature="+signature)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Permission-Timestamp", timestamp)
	request.Header.Set("X-Permission-Nonce", nonce)
	request.Header.Set("X-Permission-Content-SHA256", payloadHash)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, fmt.Errorf("Permission authorize returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var payload struct {
		Data struct {
			Permissions map[string]bool `json:"permissions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data.Permissions, nil
}
