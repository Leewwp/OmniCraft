package aliyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	green20220302 "github.com/alibabacloud-go/green-20220302/v3/client"
	"github.com/alibabacloud-go/tea/tea"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/observability"
)

var (
	ErrGreenNotConfigured = errors.New("green config is incomplete")
	// ErrGreenSeedRequired is returned when a callback is configured but
	// green.seed is empty: the official contract requires seed when using a
	// callback (the checksum cannot be computed or verified without it), so
	// submitting such a scan would silently never deliver a verifiable result.
	ErrGreenSeedRequired = errors.New("green seed is required when a callback is configured")
)

// VideoScanParams carries the VideoModeration ServiceParameters inputs.
// cryptType is intentionally never set: SHA256 is the documented default.
type VideoScanParams struct {
	VideoURL    string
	CallbackURL string
	Seed        string
	DataID      string
}

type GreenClient struct {
	accessKeyID     string
	accessKeySecret string
	region          string
}

type GreenScanResult struct {
	Result      string                 `json:"result"`
	Reason      string                 `json:"reason,omitempty"`
	RawResponse map[string]interface{} `json:"raw_response,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
}

func NewGreenClient(accessKeyID, accessKeySecret, region string) *GreenClient {
	return &GreenClient{
		accessKeyID:     strings.TrimSpace(accessKeyID),
		accessKeySecret: strings.TrimSpace(accessKeySecret),
		region:          strings.TrimSpace(region),
	}
}

func (c *GreenClient) configured() bool {
	return c != nil && c.accessKeyID != "" && c.accessKeySecret != "" && c.region != ""
}

func (c *GreenClient) newClient() (*green20220302.Client, error) {
	cfg := &openapi.Config{
		AccessKeyId:     tea.String(c.accessKeyID),
		AccessKeySecret: tea.String(c.accessKeySecret),
		RegionId:        tea.String(c.region),
		Endpoint:        tea.String(fmt.Sprintf("green-cip.%s.aliyuncs.com", c.region)),
	}
	return green20220302.NewClient(cfg)
}

func (c *GreenClient) TextModeration(ctx context.Context, text string) (result *GreenScanResult, err error) {
	started := time.Now()
	defer func() { observability.ObserveExternalCall("green", started, err) }()
	return c.textModeration(ctx, text)
}

func (c *GreenClient) textModeration(ctx context.Context, text string) (*GreenScanResult, error) {
	if !c.configured() {
		return nil, ErrGreenNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := c.newClient()
	if err != nil {
		return nil, err
	}

	serviceParams := map[string]interface{}{
		"content": text,
	}
	spJSON, err := json.Marshal(serviceParams)
	if err != nil {
		return nil, err
	}

	req := &green20220302.TextModerationPlusRequest{
		Service:           tea.String("query_security_check"),
		ServiceParameters: tea.String(string(spJSON)),
	}
	resp, err := client.TextModerationPlus(req)
	if err != nil {
		return nil, err
	}

	return parseTextModerationPlusResponse(resp)
}

func (c *GreenClient) ImageModeration(ctx context.Context, imageURL string) (result *GreenScanResult, err error) {
	started := time.Now()
	defer func() { observability.ObserveExternalCall("green", started, err) }()
	return c.imageModeration(ctx, imageURL)
}

func (c *GreenClient) imageModeration(ctx context.Context, imageURL string) (*GreenScanResult, error) {
	if !c.configured() {
		return nil, ErrGreenNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := c.newClient()
	if err != nil {
		return nil, err
	}

	spJSON, err := buildImageServiceParams(imageURL)
	if err != nil {
		return nil, err
	}

	req := &green20220302.ImageModerationRequest{
		// baselineCheck is the image moderation service accepted by the
		// green-cip ImageModeration API (verified against the real API on
		// 2026-09-01: "query_security_check" is rejected there with business
		// code 401 "parameter invalid(service)", and the wire parameter key
		// is "imageUrl", not "url").
		Service:           tea.String("baselineCheck"),
		ServiceParameters: tea.String(spJSON),
	}
	resp, err := client.ImageModeration(req)
	if err != nil {
		return nil, err
	}

	return parseImageModerationResponse(resp)
}

func (c *GreenClient) VideoAsyncScan(ctx context.Context, params VideoScanParams) (result *GreenScanResult, err error) {
	started := time.Now()
	defer func() { observability.ObserveExternalCall("green", started, err) }()
	return c.videoAsyncScan(ctx, params)
}

func (c *GreenClient) videoAsyncScan(ctx context.Context, params VideoScanParams) (*GreenScanResult, error) {
	if !c.configured() {
		return nil, ErrGreenNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	spJSON, err := buildVideoServiceParams(params)
	if err != nil {
		return nil, err
	}

	client, err := c.newClient()
	if err != nil {
		return nil, err
	}

	req := &green20220302.VideoModerationRequest{
		// videoDetection is the video file moderation service accepted by
		// the green-cip VideoModeration API (verified against the real API
		// on 2026-09-01: other names are rejected with
		// "service is invalid").
		Service:           tea.String("videoDetection"),
		ServiceParameters: tea.String(spJSON),
	}
	resp, err := client.VideoModeration(req)
	if err != nil {
		return nil, err
	}

	if resp.Body == nil {
		return &GreenScanResult{Result: "review", Reason: "video_async_submitted"}, nil
	}

	body := flattenTeaResponse(resp.Body)
	return &GreenScanResult{
		Result:      "review",
		Reason:      "video_async_submitted",
		RawResponse: body,
		TaskID:      extractTaskIDFromBody(body),
	}, nil
}

// buildImageServiceParams constructs the ServiceParameters JSON for
// ImageModeration. The baselineCheck service reads the target from
// "imageUrl"; sending the OpenAI-style "url" key yields
// "parameter is null(imageUrl)".
func buildImageServiceParams(imageURL string) (string, error) {
	serviceParams := map[string]interface{}{
		"imageUrl": strings.TrimSpace(imageURL),
	}
	spJSON, err := json.Marshal(serviceParams)
	if err != nil {
		return "", err
	}
	return string(spJSON), nil
}

// buildVideoServiceParams constructs the ServiceParameters JSON for
// VideoModeration. seed is required whenever a callback is used (official
// contract: "使用 callback 时必须提供 seed"; callback empty means polling
// mode, where seed is neither required nor sent). dataId keeps the
// {target_type}:<id> form the callback parser expects. cryptType is never
// included so the default SHA256 applies.
func buildVideoServiceParams(params VideoScanParams) (string, error) {
	serviceParams := map[string]interface{}{
		"url":      strings.TrimSpace(params.VideoURL),
		"callback": strings.TrimSpace(params.CallbackURL),
	}
	if dataID := strings.TrimSpace(params.DataID); dataID != "" {
		serviceParams["dataId"] = dataID
	}
	if callback := serviceParams["callback"].(string); callback != "" {
		seed := strings.TrimSpace(params.Seed)
		if seed == "" {
			return "", ErrGreenSeedRequired
		}
		// The format contract is owned by the config gate (config.ValidGreenSeed);
		// the request builder enforces it defensively so a misconfigured seed can
		// never reach Aliyun outside the release gate.
		if !config.ValidGreenSeed(seed) {
			return "", fmt.Errorf("green seed must be [A-Za-z0-9_], max 64 chars")
		}
		serviceParams["seed"] = seed
	}
	spJSON, err := json.Marshal(serviceParams)
	if err != nil {
		return "", err
	}
	return string(spJSON), nil
}

func (c *GreenClient) newDataID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// Response parsing

func parseTextModerationPlusResponse(resp *green20220302.TextModerationPlusResponse) (*GreenScanResult, error) {
	if resp.Body == nil {
		return nil, errors.New("empty response body")
	}
	body := flattenTeaResponse(resp.Body)

	code := intFromBody(body, "Code")
	if code != 200 {
		msg := stringFromBody(body, "Message")
		return nil, fmt.Errorf("green api error %d: %s", code, msg)
	}

	data := extractData(body)
	if data == nil {
		return &GreenScanResult{Result: "pass", Reason: "no_data", RawResponse: body}, nil
	}

	labels := stringFromMap(data, "Labels")
	riskLevel := stringFromMap(data, "RiskLevel")

	result := "pass"
	switch riskLevel {
	case "high":
		result = "block"
	case "medium":
		result = "review"
	default:
		result = "pass"
	}

	return &GreenScanResult{
		Result:      result,
		Reason:      labels,
		RawResponse: body,
	}, nil
}

func parseImageModerationResponse(resp *green20220302.ImageModerationResponse) (*GreenScanResult, error) {
	if resp.Body == nil {
		return nil, errors.New("empty response body")
	}
	body := flattenTeaResponse(resp.Body)

	code := intFromBody(body, "Code")
	if code != 200 {
		msg := stringFromBody(body, "Message")
		return nil, fmt.Errorf("green api error %d: %s", code, msg)
	}

	data := extractData(body)
	if data == nil {
		return &GreenScanResult{Result: "pass", Reason: "no_data", RawResponse: body}, nil
	}

	result, reason := imageResultFromBody(body)

	return &GreenScanResult{
		Result:      result,
		Reason:      reason,
		RawResponse: body,
	}, nil
}

// imageResultFromBody derives the normalized review result from a flattened
// baselineCheck response body. The API reports an overall Data.RiskLevel
// ("none"|"low"|"medium"|"high"); per-item entries additionally carry
// Label/RiskLevel, and the legacy Suggestion field is honored as a fallback
// so both wire shapes map onto pass/review/block.
func imageResultFromBody(body map[string]interface{}) (string, string) {
	data := extractData(body)
	if data == nil {
		return "pass", "no_data"
	}

	result, reason := parseImageResults(extractResultList(data))
	if overall := imageResultFromRiskLevel(stringFromMap(data, "RiskLevel")); rankSuggestion(overall) > rankSuggestion(result) {
		result = overall
		reason = "risk_level"
	}
	return result, reason
}

// imageResultFromRiskLevel maps an Aliyun RiskLevel onto the platform's
// normalized review result. "low" stays pass to keep false positives out of
// the human review queue; "medium" routes to review and "high" blocks,
// mirroring the text moderation mapping.
func imageResultFromRiskLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high":
		return "block"
	case "medium":
		return "review"
	default:
		return "pass"
	}
}

func parseImageResults(results []interface{}) (string, string) {
	finalResult := ""
	finalReason := ""
	finalRank := 0
	for _, r := range results {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		candidate := imageResultFromRiskLevel(stringFromMap(rm, "RiskLevel"))
		if s := stringFromMap(rm, "Suggestion"); candidate == "pass" && s != "" {
			// Legacy/callback wire shape where items carry Suggestion.
			candidate = toNormalizedResult(s)
		}
		if rank := rankSuggestion(candidate); rank > finalRank {
			finalRank = rank
			finalResult = candidate
			finalReason = stringFromMap(rm, "Label")
		}
	}
	if finalResult == "" {
		finalResult = "pass"
	}
	if finalReason == "" {
		finalReason = "image_scan"
	}
	return finalResult, finalReason
}

// Helpers

func flattenTeaResponse(body interface{}) map[string]interface{} {
	b, err := json.Marshal(body)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func extractData(body map[string]interface{}) map[string]interface{} {
	d, ok := body["Data"]
	if !ok {
		return nil
	}
	switch v := d.(type) {
	case map[string]interface{}:
		return v
	case []interface{}:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]interface{}); ok {
				return m
			}
		}
	}
	return nil
}

func extractResultList(data map[string]interface{}) []interface{} {
	r, ok := data["Result"]
	if !ok {
		return nil
	}
	switch v := r.(type) {
	case []interface{}:
		return v
	}
	return nil
}

func extractTaskIDFromBody(body map[string]interface{}) string {
	data := extractData(body)
	if data == nil {
		return ""
	}
	return stringFromMap(data, "TaskId")
}

func stringFromBody(body map[string]interface{}, key string) string {
	v, ok := body[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func intFromBody(body map[string]interface{}, key string) int {
	v, ok := body[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func stringFromMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func toNormalizedResult(suggestion string) string {
	s := strings.ToLower(strings.TrimSpace(suggestion))
	switch s {
	case "block", "violation", "reject":
		return "block"
	case "review", "suspect":
		return "review"
	default:
		return "pass"
	}
}

func rankSuggestion(suggestion string) int {
	s := strings.ToLower(strings.TrimSpace(suggestion))
	switch s {
	case "block", "violation", "reject":
		return 3
	case "review", "suspect":
		return 2
	case "pass":
		return 1
	default:
		return 0
	}
}
