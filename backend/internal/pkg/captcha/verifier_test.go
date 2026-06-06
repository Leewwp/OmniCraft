package captcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
)

func TestAliyunV2VerifierRejectsEmptyToken(t *testing.T) {
	verifier := NewAliyunV2Verifier(config.CaptchaConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		SceneID:         "scene-id",
		Region:          "cn",
	})

	err := verifier.Verify(context.Background(), "", "203.0.113.10")

	require.Error(t, err)
	require.Contains(t, err.Error(), "captcha token is required")
}

func TestAliyunV2VerifierSendsIntelligentCaptchaParams(t *testing.T) {
	var capturedAction string
	var capturedSceneID string
	var capturedVerifyParam string
	var capturedRemoteIP string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		capturedAction = firstParam(r, "Action")
		capturedSceneID = firstParam(r, "SceneId")
		capturedVerifyParam = firstParam(r, "CaptchaVerifyParam")
		capturedRemoteIP = firstParam(r, "RemoteIp")

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Code":      "OK",
			"RequestId": "req-success",
			"Data": map[string]interface{}{
				"CaptchaVerifyResult": true,
			},
		}))
	}))
	defer server.Close()

	verifier := NewAliyunV2Verifier(config.CaptchaConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		SceneID:         "scene-id",
		Region:          "cn",
	})
	verifier.endpoint = server.URL

	err := verifier.Verify(context.Background(), "captcha-verify-param", "203.0.113.10")

	require.NoError(t, err)
	require.Equal(t, "VerifyIntelligentCaptcha", capturedAction)
	require.Equal(t, "scene-id", capturedSceneID)
	require.Equal(t, "captcha-verify-param", capturedVerifyParam)
	require.Equal(t, "203.0.113.10", capturedRemoteIP)
}

func TestAliyunV2VerifierReturnsErrorWhenAliyunFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Code":      "OK",
			"RequestId": "req-failed",
			"Data": map[string]interface{}{
				"CaptchaVerifyResult": false,
			},
		}))
	}))
	defer server.Close()

	verifier := NewAliyunV2Verifier(config.CaptchaConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		SceneID:         "scene-id",
		Region:          "cn",
	})
	verifier.endpoint = server.URL

	err := verifier.Verify(context.Background(), "captcha-verify-param", "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "captcha verification failed")
}

func TestAliyunV2VerifierReturnsNilWhenAliyunSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"Code":      "OK",
			"RequestId": "req-success",
			"Data": map[string]interface{}{
				"CaptchaVerifyResult": true,
			},
		}))
	}))
	defer server.Close()

	verifier := NewAliyunV2Verifier(config.CaptchaConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		SceneID:         "scene-id",
		Region:          "cn",
	})
	verifier.endpoint = server.URL

	err := verifier.Verify(context.Background(), "captcha-verify-param", "")

	require.NoError(t, err)
}

func firstParam(r *http.Request, key string) string {
	values := r.Form[key]
	if len(values) == 0 {
		values = r.URL.Query()[key]
	}
	if len(values) == 0 {
		for candidate, vals := range r.Form {
			if strings.EqualFold(candidate, key) && len(vals) > 0 {
				return vals[0]
			}
		}
		return ""
	}
	return values[0]
}
