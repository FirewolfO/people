package edge

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const publicPrefix = "/api/open/people"

type Proxy struct {
	target    *url.URL
	accessKey string
	secretKey string
	proxy     *httputil.ReverseProxy
}

func New(target, accessKey, secretKey string) (*Proxy, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(target), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid people upstream URL")
	}
	if strings.TrimSpace(accessKey) == "" || len(secretKey) < 16 {
		return nil, fmt.Errorf("gateway credentials are required")
	}
	result := &Proxy{target: parsed, accessKey: accessKey, secretKey: secretKey}
	result.proxy = &httputil.ReverseProxy{
		Rewrite: result.rewrite,
		ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(writer, `{"code":"UPSTREAM_UNAVAILABLE","message":"People 服务暂不可用"}`, http.StatusBadGateway)
		},
	}
	return result, nil
}

func (p *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/health" {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
		return
	}
	if request.URL.Path != publicPrefix && !strings.HasPrefix(request.URL.Path, publicPrefix+"/") {
		http.NotFound(writer, request)
		return
	}
	if request.ContentLength > 10<<20 {
		http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, (10<<20)+1))
	if err != nil || len(body) > 10<<20 {
		http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request = request.WithContext(withBody(request.Context(), body))
	p.proxy.ServeHTTP(writer, request)
}

func (p *Proxy) rewrite(out *httputil.ProxyRequest) {
	body, _ := bodyFrom(out.In.Context())
	rest := strings.TrimPrefix(out.In.URL.Path, publicPrefix)
	if rest == "" {
		rest = "/"
	}
	out.SetURL(p.target)
	out.Out.URL.Path = strings.TrimRight(p.target.Path, "/") + "/api/v1" + rest
	out.Out.URL.RawPath = ""
	out.Out.Host = p.target.Host
	for name := range out.Out.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-gateway-") {
			out.Out.Header.Del(name)
		}
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])
	canonical := strings.Join([]string{
		out.Out.Method,
		out.Out.URL.EscapedPath(),
		canonicalQuery(out.Out.URL.Query()),
		timestamp,
		nonce,
		payloadHash,
	}, "\n")
	mac := hmac.New(sha256.New, []byte(p.secretKey))
	_, _ = mac.Write([]byte(canonical))
	out.Out.Header.Set("X-Gateway-Credential", p.accessKey)
	out.Out.Header.Set("X-Gateway-Signature", hex.EncodeToString(mac.Sum(nil)))
	out.Out.Header.Set("X-Gateway-Timestamp", timestamp)
	out.Out.Header.Set("X-Gateway-Nonce", nonce)
	out.Out.Header.Set("X-Gateway-Content-SHA256", payloadHash)
}

func canonicalQuery(values url.Values) string {
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
	return strings.Join(parts, "&")
}

func encode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
