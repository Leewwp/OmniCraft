package aliyun

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
