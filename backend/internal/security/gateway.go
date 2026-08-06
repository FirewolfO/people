package security

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	credentialHeader = "X-Gateway-Credential"
	signatureHeader  = "X-Gateway-Signature"
	timestampHeader  = "X-Gateway-Timestamp"
	nonceHeader      = "X-Gateway-Nonce"
	payloadHeader    = "X-Gateway-Content-SHA256"
)

type GatewayVerifier struct {
	accessKey string
	secretKey string
	skew      time.Duration
	mu        sync.Mutex
	nonces    map[string]time.Time
}

func NewGatewayVerifier(accessKey, secretKey string, skew time.Duration) *GatewayVerifier {
	return &GatewayVerifier{accessKey: accessKey, secretKey: secretKey, skew: skew, nonces: make(map[string]time.Time)}
}

func (v *GatewayVerifier) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !v.verify(c) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
				"code": "INVALID_GATEWAY_SIGNATURE", "message": "Gateway 调用认证失败",
			})
			return
		}
		c.Next(ctx)
	}
}

func (v *GatewayVerifier) verify(c *app.RequestContext) bool {
	credential := string(c.Request.Header.Peek(credentialHeader))
	timestamp := string(c.Request.Header.Peek(timestampHeader))
	nonce := string(c.Request.Header.Peek(nonceHeader))
	payloadHash := strings.ToLower(string(c.Request.Header.Peek(payloadHeader)))
	signature := strings.ToLower(string(c.Request.Header.Peek(signatureHeader)))
	if !equal(v.accessKey, credential) || len(nonce) < 16 || len(nonce) > 128 || len(payloadHash) != 64 {
		return false
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	requestTime := time.Unix(seconds, 0).UTC()
	if requestTime.Before(now.Add(-v.skew)) || requestTime.After(now.Add(v.skew)) {
		return false
	}
	body := c.Request.Body()
	sum := sha256.Sum256(body)
	actualPayloadHash := hex.EncodeToString(sum[:])
	if !equal(actualPayloadHash, payloadHash) {
		return false
	}
	query, err := canonicalQuery(string(c.Request.URI().QueryString()))
	if err != nil {
		return false
	}
	path := string(c.Request.URI().PathOriginal())
	canonical := strings.Join([]string{strings.ToUpper(string(c.Method())), path, query, timestamp, nonce, payloadHash}, "\n")
	mac := hmac.New(sha256.New, []byte(v.secretKey))
	_, _ = mac.Write([]byte(canonical))
	if !equal(hex.EncodeToString(mac.Sum(nil)), signature) {
		return false
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for key, expiresAt := range v.nonces {
		if !expiresAt.After(now) {
			delete(v.nonces, key)
		}
	}
	key := credential + ":" + nonce
	if _, exists := v.nonces[key]; exists {
		return false
	}
	v.nonces[key] = requestTime.Add(v.skew)
	return true
}

func canonicalQuery(raw string) (string, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0)
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		if len(items) == 0 {
			items = []string{""}
		}
		for _, item := range items {
			parts = append(parts, encode(key)+"="+encode(item))
		}
	}
	return strings.Join(parts, "&"), nil
}

func encode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

func equal(expected, actual string) bool {
	return len(expected) == len(actual) && subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}
