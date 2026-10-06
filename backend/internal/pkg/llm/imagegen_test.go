package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCogViewClientURLRoundTrip(t *testing.T) {
	var gotPath string
	var gotBody ImageGenerateRequest
	var gotAuth string
	imageBytes := []byte("fake-png-body")
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageBytes)
	}))
	defer dl.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": dl.URL + "/img.png"}},
		})
	}))
	defer srv.Close()

	c := NewCogViewClient(srv.URL, "sk-test", "cogview-4")
	c.AllowPrivateHosts = true // httptest servers live on 127.0.0.1
	out, err := c.GenerateImage(context.Background(), "灯塔夜景", "1024x1024", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/images/generations" {
		t.Fatalf("endpoint path = %s", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotBody.Model != "cogview-4" || gotBody.Prompt != "灯塔夜景" || gotBody.Size != "1024x1024" {
		t.Fatalf("request body = %+v", gotBody)
	}
	if string(out) != string(imageBytes) {
		t.Fatalf("downloaded bytes mismatch")
	}
}

func TestCogViewClientB64Payload(t *testing.T) {
	payload := []byte("b64-image-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString(payload)}},
		})
	}))
	defer srv.Close()
	c := NewCogViewClient(srv.URL, "k", "cogview-4")
	c.AllowPrivateHosts = true // httptest server on 127.0.0.1
	out, err := c.GenerateImage(context.Background(), "p", "", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(payload) {
		t.Fatalf("b64 decode mismatch")
	}
}

func TestCogViewClientLimitsAndErrors(t *testing.T) {
	// oversize download is rejected, not truncated silently
	big := strings.Repeat("x", 100)
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(big))
	}))
	defer dl.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": dl.URL + "/big.png"}},
		})
	}))
	defer srv.Close()
	c := NewCogViewClient(srv.URL, "k", "cogview-4")
	c.AllowPrivateHosts = true // httptest servers on 127.0.0.1
	if _, err := c.GenerateImage(context.Background(), "p", "", 10); err == nil || !strings.Contains(err.Error(), "max bytes") {
		t.Fatalf("oversize must be rejected: %v", err)
	}

	// provider error payload surfaces
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "bad prompt"}})
	}))
	defer errSrv.Close()
	c2 := NewCogViewClient(errSrv.URL, "k", "cogview-4")
	c2.AllowPrivateHosts = true
	if _, err := c2.GenerateImage(context.Background(), "p", "", 1<<20); err == nil || !strings.Contains(err.Error(), "bad prompt") {
		t.Fatalf("provider error must surface: %v", err)
	}

	// empty data array
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer emptySrv.Close()
	c3 := NewCogViewClient(emptySrv.URL, "k", "cogview-4")
	c3.AllowPrivateHosts = true
	if _, err := c3.GenerateImage(context.Background(), "p", "", 1<<20); err == nil || !strings.Contains(err.Error(), "no image") {
		t.Fatalf("empty data must fail: %v", err)
	}
}

// SP-25 低-11：生图结果下载的主机校验（SSRF 纵深）。
func TestGuardDownloadHost(t *testing.T) {
	c := NewCogViewClient("https://open.bigmodel.cn", "sk-test", "cogview-4")
	mustURL := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"public https host", "https://cdn.example.com/img.png", true},
		{"provider api_base host trusted", "https://open.bigmodel.cn/file.png", true},
		{"cloud metadata endpoint", "http://169.254.169.254/latest/meta-data", false},
		{"aliyun imds via cgnat range", "http://100.100.100.200/latest/meta-data", false},
		{"cgnat range low edge", "http://100.64.0.1/img.png", false},
		{"cgnat range high edge", "http://100.127.255.255/img.png", false},
		{"just above cgnat range is public", "http://100.128.0.1/img.png", true},
		{"private range v4", "http://10.0.0.5/img.png", false},
		{"private range v6", "http://[fd00::1]/img.png", false},
		{"loopback literal", "http://127.0.0.1:8080/img.png", false},
		{"localhost name", "http://localhost/img.png", false},
		{"trailing-dot localhost fqdn", "http://localhost.:8080/img.png", false},
		{"trailing-dot internal fqdn", "https://db.internal./img.png", false},
		{"internal suffix", "https://db.internal/img.png", false},
		{"non-http scheme", "ftp://example.com/img.png", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.guardDownloadHost(mustURL(tc.raw))
			if tc.ok && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected rejection for %s", tc.raw)
			}
		})
	}
}

func TestGuardDownloadHostPrivateAllowedForTests(t *testing.T) {
	c := NewCogViewClient("http://127.0.0.1:1", "sk-test", "cogview-4")
	c.AllowPrivateHosts = true
	u, _ := url.Parse("http://10.1.2.3/img.png")
	if err := c.guardDownloadHost(u); err != nil {
		t.Fatalf("AllowPrivateHosts bypass must relax the guard, got %v", err)
	}
}

func TestGenerateImageRejectsMetadataRedirect(t *testing.T) {
	// provider 返回指向云元数据端点的 URL：下载必须被守卫拒绝，
	// 而不是把内网字节经 OSS 外带。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": "http://169.254.169.254/latest/meta-data"}},
		})
	}))
	defer srv.Close()

	c := NewCogViewClient(srv.URL, "sk-test", "cogview-4")
	_, err := c.GenerateImage(context.Background(), "灯塔", "1024x1024", 1<<20)
	if err == nil {
		t.Fatal("metadata-endpoint image url must be rejected")
	}
	if !strings.Contains(err.Error(), "private/loopback/link-local") {
		t.Fatalf("error should name the host guard, got %v", err)
	}
}

// TestGuardDownloadHostAPIBasePortBinding（审计 #8 / #814）：api_base 信任锚
// 必须比对 host:port（及 scheme 隐含的默认端口），不能只比 hostname——否则
// 攻击者控制的 provider 可以用「同 host 异端口」把下载引向任意地址。锚点
// host 取环回地址，端口差异才可见：同 host 异端口不再豁免。
func TestGuardDownloadHostAPIBasePortBinding(t *testing.T) {
	mustURL := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	c := NewCogViewClient("http://127.0.0.1:8443", "sk-test", "cogview-4")

	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"same host and port trusted", "http://127.0.0.1:8443/file.png", true},
		{"same host different port", "http://127.0.0.1:9443/file.png", false},
		{"same host scheme-implied default port", "http://127.0.0.1/file.png", false},
		{"scheme upgrade implies different default port", "https://127.0.0.1:8443/file.png", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.guardDownloadHost(mustURL(tc.raw))
			if tc.ok && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("expected rejection for %s", tc.raw)
			}
		})
	}

	// Default ports equal explicit well-known ports for the same scheme.
	c2 := NewCogViewClient("http://127.0.0.1:80", "sk-test", "cogview-4")
	if err := c2.guardDownloadHost(mustURL("http://127.0.0.1/file.png")); err != nil {
		t.Fatalf("explicit 80 must match the http default port, got %v", err)
	}
	c3 := NewCogViewClient("https://127.0.0.1:443", "sk-test", "cogview-4")
	if err := c3.guardDownloadHost(mustURL("https://127.0.0.1/file.png")); err != nil {
		t.Fatalf("explicit 443 must match the https default port, got %v", err)
	}
}

// TestGenerateImageRejectsRedirectToLoopback（审计 #8 / #814）：首跳 URL 是
// api_base 信任锚（合法域名同 host:port），但 302 跳向环回地址——每一跳都
// 必须过守卫，而不是只有第一跳。
func TestGenerateImageRejectsRedirectToLoopback(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "http://127.0.0.1:1/leak.png")
			w.WriteHeader(http.StatusFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"url": srv.URL + "/redirect"}},
		})
	}))
	defer srv.Close()

	c := NewCogViewClient(srv.URL, "sk-test", "cogview-4")
	_, err := c.GenerateImage(context.Background(), "灯塔", "1024x1024", 1<<20)
	if err == nil {
		t.Fatal("redirect to a loopback host must be rejected, not followed")
	}
	if !strings.Contains(err.Error(), "private/loopback/link-local") {
		t.Fatalf("error should name the host guard instead of a dial failure, got %v", err)
	}
}

// TestRedirectPolicyGuardsEveryHop：redirectPolicy 可直接单测（构造 req/via
// 切片，无网络）：每一跳都过下载主机守卫，跳数超限被拒。
func TestRedirectPolicyGuardsEveryHop(t *testing.T) {
	c := NewCogViewClient("https://api.example.com", "sk-test", "cogview-4")
	mkReq := func(raw string) *http.Request {
		req, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return req
	}

	// First hop to a public host is allowed.
	if err := c.redirectPolicy(mkReq("https://cdn.example.com/a.png"), nil); err != nil {
		t.Fatalf("public redirect hop must pass, got %v", err)
	}
	// A later hop to a metadata endpoint is rejected even though the chain
	// started at a trusted host.
	via := []*http.Request{mkReq("https://cdn.example.com/a.png")}
	if err := c.redirectPolicy(mkReq("http://169.254.169.254/latest/meta-data"), via); err == nil ||
		!strings.Contains(err.Error(), "private/loopback/link-local") {
		t.Fatalf("metadata redirect hop must be rejected, got %v", err)
	}
	if err := c.redirectPolicy(mkReq("http://100.100.100.200/latest/meta-data"), via); err == nil {
		t.Fatal("cgnat metadata redirect hop must be rejected")
	}
	// The api_base origin stays trusted on every hop.
	if err := c.redirectPolicy(mkReq("https://api.example.com/file.png"), via); err != nil {
		t.Fatalf("api_base origin hop must pass, got %v", err)
	}
	// Hop cap: the 6th hop is rejected regardless of target.
	capped := make([]*http.Request, 5)
	for i := range capped {
		capped[i] = mkReq("https://cdn.example.com/hop")
	}
	if err := c.redirectPolicy(mkReq("https://cdn.example.com/next"), capped); err == nil ||
		!strings.Contains(err.Error(), "redirect hops") {
		t.Fatalf("hop cap must be enforced, got %v", err)
	}
}

// TestGuardDialAddress：拨号层守卫可直接单测——私网/环回/CGNAT 字面量被拒，
// 公网地址放行。
func TestGuardDialAddress(t *testing.T) {
	rejected := []string{
		"127.0.0.1:80",
		"[::1]:443",
		"10.1.2.3:8080",
		"169.254.169.254:80",
		"100.100.100.200:80",
		"0.0.0.0:80",
	}
	for _, addr := range rejected {
		if err := guardDialAddress(addr); err == nil {
			t.Fatalf("dial to %s must be rejected", addr)
		}
	}
	if err := guardDialAddress("93.184.216.34:443"); err != nil {
		t.Fatalf("public dial must pass, got %v", err)
	}
	if err := guardDialAddress("not-an-ip:443"); err == nil {
		t.Fatal("non-IP dial address must be rejected")
	}
}
