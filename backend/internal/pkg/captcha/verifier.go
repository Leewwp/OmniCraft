package captcha

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"omnicraft/backend/config"
)

type CaptchaVerifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

type BypassVerifier struct{}

func NewBypassVerifier() *BypassVerifier {
	return &BypassVerifier{}
}

func (v *BypassVerifier) Verify(_ context.Context, _, _ string) error {
	return nil
}

type AliyunV2Verifier struct {
	accessKeyID     string
	accessKeySecret string
	sceneID         string
	prefix          string
	region          string
	endpoint        string
	httpClient      *http.Client
}

func NewAliyunV2Verifier(cfg config.CaptchaConfig) *AliyunV2Verifier {
	region := cfg.Region
	if region == "" {
		region = "cn"
	}
	return &AliyunV2Verifier{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
		sceneID:         cfg.SceneID,
		prefix:          cfg.Prefix,
		region:          region,
		endpoint:        aliyunCaptchaEndpoint(region),
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *AliyunV2Verifier) Verify(ctx context.Context, token, remoteIP string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("captcha token is required")
	}
	if strings.TrimSpace(v.accessKeyID) == "" || strings.TrimSpace(v.accessKeySecret) == "" || strings.TrimSpace(v.sceneID) == "" {
		return fmt.Errorf("captcha verifier is not configured")
	}

	params := map[string]string{
		"Action":             "VerifyIntelligentCaptcha",
		"Version":            "2023-03-05",
		"Format":             "JSON",
		"AccessKeyId":        v.accessKeyID,
		"SignatureMethod":    "HMAC-SHA1",
		"SignatureVersion":   "1.0",
		"SignatureNonce":     newAliyunNonce(),
		"Timestamp":          time.Now().UTC().Format(time.RFC3339),
		"SceneId":            v.sceneID,
		"CaptchaVerifyParam": token,
	}
	if remoteIP != "" {
		params["RemoteIp"] = remoteIP
	}
	params["Signature"] = signAliyunRPC(params, v.accessKeySecret)

	formBody := encodeAliyunRPCParams(params)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(formBody))
	if err != nil {
		return fmt.Errorf("failed to create captcha verify request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-acs-action", "VerifyIntelligentCaptcha")
	req.Header.Set("x-acs-version", "2023-03-05")

	slog.Debug("captcha verify request", "scene", v.sceneID, "remote_ip", remoteIP)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("captcha verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("captcha verify returned non-200 status", "status", resp.StatusCode)
		return fmt.Errorf("captcha verify returned status %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("failed to decode captcha verify response: %w", err)
	}
	aliyunResp := parseAliyunCaptchaVerifyResponse(body)

	if !isAliyunCaptchaSuccessCode(aliyunResp.Code) {
		slog.Warn("captcha verify api error", "code", aliyunResp.Code, "request_id", aliyunResp.RequestID, "message", aliyunResp.Message)
		return fmt.Errorf("captcha verification failed")
	}
	if !aliyunResp.verifyResult() {
		slog.Warn("captcha verify rejected token", "code", aliyunResp.Code, "request_id", aliyunResp.RequestID, "verify_code", aliyunResp.verifyCode())
		return fmt.Errorf("captcha verification failed")
	}
	slog.Info("captcha verify success", "code", aliyunResp.Code, "request_id", aliyunResp.RequestID, "verify_result", true, "verify_code", aliyunResp.verifyCode())

	return nil
}

type aliyunCaptchaVerifyResponse struct {
	Code      string                 `json:"Code"`
	Message   string                 `json:"Message"`
	RequestID string                 `json:"RequestId"`
	Data      map[string]interface{} `json:"Data"`
	Result    map[string]interface{} `json:"Result"`
}

func parseAliyunCaptchaVerifyResponse(body map[string]interface{}) aliyunCaptchaVerifyResponse {
	resp := aliyunCaptchaVerifyResponse{
		Code:      stringFromAliyunMap(body, "Code"),
		Message:   stringFromAliyunMap(body, "Message"),
		RequestID: stringFromAliyunMap(body, "RequestId"),
		Data:      map[string]interface{}{},
		Result:    map[string]interface{}{},
	}
	if data, ok := body["Data"].(map[string]interface{}); ok {
		resp.Data = data
	}
	if result, ok := body["Result"].(map[string]interface{}); ok {
		resp.Result = result
	} else if value, ok := body["Result"]; ok {
		resp.Data["Result"] = value
	}
	for _, key := range []string{"CaptchaVerifyResult", "VerifyResult", "VerifyCode"} {
		if value, ok := body[key]; ok {
			resp.Data[key] = value
		}
	}
	return resp
}

func (r aliyunCaptchaVerifyResponse) verifyResult() bool {
	for _, values := range []map[string]interface{}{r.Result, r.Data} {
		for _, key := range []string{"CaptchaVerifyResult", "VerifyResult", "Result"} {
			if value, ok := values[key]; ok {
				return boolFromAliyunValue(value)
			}
		}
	}
	return false
}

func (r aliyunCaptchaVerifyResponse) verifyCode() string {
	for _, values := range []map[string]interface{}{r.Result, r.Data} {
		for _, key := range []string{"VerifyCode", "CaptchaVerifyCode"} {
			if value, ok := values[key]; ok {
				return stringFromAliyunValue(value)
			}
		}
	}
	return ""
}

func isAliyunCaptchaSuccessCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "ok", "success":
		return true
	default:
		return false
	}
}

func boolFromAliyunValue(value interface{}) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func aliyunCaptchaEndpoint(region string) string {
	normalized := strings.TrimSpace(strings.ToLower(region))
	switch normalized {
	case "", "cn", "cn-shanghai":
		return "https://captcha.cn-shanghai.aliyuncs.com"
	default:
		return fmt.Sprintf("https://captcha.%s.aliyuncs.com", normalized)
	}
}

func newAliyunNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func signAliyunRPC(params map[string]string, accessKeySecret string) string {
	canonical := canonicalAliyunRPCQuery(params)
	stringToSign := "POST&%2F&" + aliyunPercentEncode(canonical)
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func encodeAliyunRPCParams(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

func canonicalAliyunRPCQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if key != "Signature" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	return strings.Join(parts, "&")
}

func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func stringFromAliyunMap(body map[string]interface{}, key string) string {
	value, ok := body[key]
	if !ok {
		return ""
	}
	return stringFromAliyunValue(value)
}

func stringFromAliyunValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func NewCaptchaVerifier(cfg config.CaptchaConfig) CaptchaVerifier {
	switch cfg.Provider {
	case "aliyun_v2":
		return NewAliyunV2Verifier(cfg)
	case "bypass", "":
		return NewBypassVerifier()
	default:
		slog.Warn("unknown captcha provider, falling back to bypass", "provider", cfg.Provider)
		return NewBypassVerifier()
	}
}
