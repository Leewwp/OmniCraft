package aliyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testCallbackURL = "https://api.leeppp.online/api/v1/internal/ai-callback"

func decodeServiceParams(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		t.Fatalf("ServiceParameters must be valid JSON: %v", err)
	}
	return params
}

func TestBuildVideoServiceParamsIncludesSeedAndDataID(t *testing.T) {
	raw, err := buildVideoServiceParams(VideoScanParams{
		VideoURL:    "https://cdn.example.test/uploads/42/video/v.mp4",
		CallbackURL: testCallbackURL,
		Seed:        "seed_abc_123",
		DataID:      "content:42",
	})
	if err != nil {
		t.Fatalf("buildVideoServiceParams: %v", err)
	}
	params := decodeServiceParams(t, raw)

	if got := params["url"]; got != "https://cdn.example.test/uploads/42/video/v.mp4" {
		t.Fatalf("url = %v", got)
	}
	if got := params["callback"]; got != testCallbackURL {
		t.Fatalf("callback = %v", got)
	}
	if got := params["seed"]; got != "seed_abc_123" {
		t.Fatalf("seed = %v, want seed_abc_123", got)
	}
	if got := params["dataId"]; got != "content:42" {
		t.Fatalf("dataId = %v, want content:42", got)
	}
	if _, ok := params["cryptType"]; ok {
		t.Fatal("cryptType must stay unset so the default SHA256 applies")
	}
}

func TestBuildVideoServiceParamsEmptySeedWithCallbackReturnsError(t *testing.T) {
	_, err := buildVideoServiceParams(VideoScanParams{
		VideoURL:    "https://cdn.example.test/uploads/42/video/v.mp4",
		CallbackURL: testCallbackURL,
		Seed:        "",
		DataID:      "content:42",
	})
	if !errors.Is(err, ErrGreenSeedRequired) {
		t.Fatalf("err = %v, want ErrGreenSeedRequired (official contract: seed is required when using callback)", err)
	}
}

func TestBuildVideoServiceParamsPollingModeOmitsSeed(t *testing.T) {
	raw, err := buildVideoServiceParams(VideoScanParams{
		VideoURL: "https://cdn.example.test/uploads/42/video/v.mp4",
		DataID:   "content:42",
	})
	if err != nil {
		t.Fatalf("empty callback (polling mode) must not require a seed: %v", err)
	}
	params := decodeServiceParams(t, raw)
	if _, ok := params["seed"]; ok {
		t.Fatal("seed must not be sent in polling mode (callback empty)")
	}
	if got := params["dataId"]; got != "content:42" {
		t.Fatalf("dataId = %v, want content:42", got)
	}
}

func TestBuildVideoServiceParamsDataIDRespectsSafeCharset(t *testing.T) {
	// Spec decision A3: dataId is the generic {target_type}:<id> form
	// ("content:<id>"), which the callback parser splits on the first colon.
	// Both halves must be safe characters and the total length is bounded by 64.
	dataID := "content:9223372036854775807"
	if len(dataID) > 64 {
		t.Fatalf("dataId %q length %d exceeds 64", dataID, len(dataID))
	}
	raw, err := buildVideoServiceParams(VideoScanParams{
		VideoURL:    "https://cdn.example.test/uploads/42/video/v.mp4",
		CallbackURL: testCallbackURL,
		Seed:        "seed",
		DataID:      dataID,
	})
	if err != nil {
		t.Fatalf("buildVideoServiceParams: %v", err)
	}
	params := decodeServiceParams(t, raw)
	if got := params["dataId"]; got != dataID {
		t.Fatalf("dataId = %v, want %q", got, dataID)
	}
}

func TestBuildVideoServiceParamsSeedCharset(t *testing.T) {
	// seed must stay within [A-Za-z0-9_], length <= 64; anything else is a
	// configuration defect that must fail before the request is sent.
	_, err := buildVideoServiceParams(VideoScanParams{
		VideoURL:    "https://cdn.example.test/uploads/42/video/v.mp4",
		CallbackURL: testCallbackURL,
		Seed:        strings.Repeat("s", 65),
		DataID:      "content:1",
	})
	if err == nil {
		t.Fatal("overlong seed must be rejected")
	}
}

func TestBuildImageServiceParamsUsesImageUrlKey(t *testing.T) {
	spJSON, err := buildImageServiceParams("https://cdn.example.test/probe/x.png")
	require.NoError(t, err)
	require.Contains(t, spJSON, `"imageUrl"`)
	require.NotContains(t, spJSON, `"url"`)
}

func TestImageResultFromBodyMapsRiskLevels(t *testing.T) {
	item := func(label, riskLevel string) map[string]interface{} {
		return map[string]interface{}{"Label": label, "RiskLevel": riskLevel}
	}
	cases := []struct {
		name string
		body map[string]interface{}
		want string
	}{
		{
			name: "overall none maps to pass",
			body: map[string]interface{}{
				"Code": 200,
				"Data": map[string]interface{}{"RiskLevel": "none", "Result": []interface{}{item("nonLabel", "none")}},
			},
			want: "pass",
		},
		{
			name: "overall medium maps to review",
			body: map[string]interface{}{
				"Code": 200,
				"Data": map[string]interface{}{"RiskLevel": "medium", "Result": []interface{}{item("porn", "medium")}},
			},
			want: "review",
		},
		{
			name: "overall high maps to block even when an item reads none",
			body: map[string]interface{}{
				"Code": 200,
				"Data": map[string]interface{}{"RiskLevel": "high", "Result": []interface{}{item("contraband_act", "none")}},
			},
			want: "block",
		},
		{
			name: "per-item risk escalates when overall is missing",
			body: map[string]interface{}{
				"Code": 200,
				"Data": map[string]interface{}{"Result": []interface{}{item("nonLabel", "none"), item("politics", "high")}},
			},
			want: "block",
		},
		{
			name: "legacy suggestion shape still recognized",
			body: map[string]interface{}{
				"Code": 200,
				"Data": map[string]interface{}{"Result": []interface{}{map[string]interface{}{"Label": "porn", "Suggestion": "block"}}},
			},
			want: "block",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := imageResultFromBody(tc.body)
			require.Equal(t, tc.want, got)
		})
	}
}
