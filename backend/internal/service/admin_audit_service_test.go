package service

import (
	"testing"
)

func TestFilterMetadata_AllowlistStripsUnknownKeys(t *testing.T) {
	raw := map[string]interface{}{
		"content_id": "123",
		"reason":     "spam",
		"author_id":  "456",
		"token":      "should-be-removed",
		"extra":      "should-be-removed",
	}
	filtered := filterMetadata("content_ban", raw)

	if _, ok := filtered["token"]; ok {
		t.Error("sensitive key 'token' should be stripped")
	}
	if _, ok := filtered["extra"]; ok {
		t.Error("non-allowlisted key 'extra' should be stripped")
	}
	if filtered["content_id"] != "123" {
		t.Errorf("expected content_id=123, got %v", filtered["content_id"])
	}
	if filtered["reason"] != "spam" {
		t.Errorf("expected reason=spam, got %v", filtered["reason"])
	}
}

func TestFilterMetadata_SensitiveKeysAlwaysStripped(t *testing.T) {
	raw := map[string]interface{}{
		"password":     "secret123",
		"api_key":      "key123",
		"cookie":       "session=abc",
		"access_key":   "ak123",
		"private_key":  "pk123",
		"authorization": "Bearer xyz",
		"header":       "X-Custom: val",
		"grant":        "g123",
		"secret":       "s123",
	}
	filtered := filterMetadata("content_ban", raw)

	for _, key := range []string{"password", "api_key", "cookie", "access_key", "private_key", "authorization", "header", "grant", "secret"} {
		if _, ok := filtered[key]; ok {
			t.Errorf("sensitive key '%s' should be stripped", key)
		}
	}
}

func TestFilterMetadata_NoAllowlistPassesNonSensitive(t *testing.T) {
	raw := map[string]interface{}{
		"custom_field": "value",
		"token":        "should-be-removed",
	}
	filtered := filterMetadata("unknown_action", raw)

	if _, ok := filtered["token"]; ok {
		t.Error("sensitive key 'token' should be stripped even without allowlist")
	}
	if filtered["custom_field"] != "value" {
		t.Errorf("expected custom_field=value, got %v", filtered["custom_field"])
	}
}

func TestFilterMetadata_NilInput(t *testing.T) {
	filtered := filterMetadata("content_ban", nil)
	if len(filtered) != 0 {
		t.Errorf("expected empty map, got %v", filtered)
	}
}

func TestFilterMetadata_CaseInsensitiveSensitiveDetection(t *testing.T) {
	raw := map[string]interface{}{
		"API_KEY":    "should-be-removed",
		"Password":   "should-be-removed",
		"COOKIE_VAL": "should-be-removed",
		"valid_key":  "kept",
	}
	filtered := filterMetadata("unknown_action", raw)

	if _, ok := filtered["API_KEY"]; ok {
		t.Error("case-insensitive sensitive key 'API_KEY' should be stripped")
	}
	if _, ok := filtered["Password"]; ok {
		t.Error("case-insensitive sensitive key 'Password' should be stripped")
	}
	if _, ok := filtered["COOKIE_VAL"]; ok {
		t.Error("case-insensitive sensitive key 'COOKIE_VAL' should be stripped")
	}
	if filtered["valid_key"] != "kept" {
		t.Errorf("expected valid_key=kept, got %v", filtered["valid_key"])
	}
}

func TestFilterMetadata_ConfigPatchMasksValues(t *testing.T) {
	raw := map[string]interface{}{
		"field":            "smtp.password",
		"old_value_masked": "***",
		"new_value_masked": "***",
		"raw_old_value":    "actual-secret",
	}
	filtered := filterMetadata("config_patch", raw)

	if _, ok := filtered["raw_old_value"]; ok {
		t.Error("non-allowlisted key 'raw_old_value' should be stripped for config_patch")
	}
	if filtered["field"] != "smtp.password" {
		t.Errorf("expected field=smtp.password, got %v", filtered["field"])
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"token", true},
		{"refresh_token", true},
		{"api_key", true},
		{"password", true},
		{"cookie", true},
		{"secret", true},
		{"grant", true},
		{"access_key_id", true},
		{"private_key", true},
		{"authorization", true},
		{"header", true},
		{"content_id", false},
		{"reason", false},
		{"name", false},
		{"slug", false},
		{"order", false},
	}
	for _, tt := range tests {
		result := isSensitiveKey(tt.key)
		if result != tt.expected {
			t.Errorf("isSensitiveKey(%q) = %v, want %v", tt.key, result, tt.expected)
		}
	}
}
