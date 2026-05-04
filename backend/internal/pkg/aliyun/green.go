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
)

var ErrGreenNotConfigured = errors.New("green config is incomplete")

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

func (c *GreenClient) TextModeration(ctx context.Context, text string) (*GreenScanResult, error) {
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

func (c *GreenClient) ImageModeration(ctx context.Context, imageURL string) (*GreenScanResult, error) {
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
		"url": imageURL,
	}
	spJSON, err := json.Marshal(serviceParams)
	if err != nil {
		return nil, err
	}

	req := &green20220302.ImageModerationRequest{
		Service:           tea.String("query_security_check"),
		ServiceParameters: tea.String(string(spJSON)),
	}
	resp, err := client.ImageModeration(req)
	if err != nil {
		return nil, err
	}

	return parseImageModerationResponse(resp)
}

func (c *GreenClient) VideoAsyncScan(ctx context.Context, videoURL, callbackURL string) (*GreenScanResult, error) {
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
		"url":      videoURL,
		"callback": callbackURL,
	}
	spJSON, err := json.Marshal(serviceParams)
	if err != nil {
		return nil, err
	}

	req := &green20220302.VideoModerationRequest{
		Service:           tea.String("query_security_check"),
		ServiceParameters: tea.String(string(spJSON)),
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

	resultList := extractResultList(data)
	result, reason := parseImageResults(resultList)

	return &GreenScanResult{
		Result:      result,
		Reason:      reason,
		RawResponse: body,
	}, nil
}

func parseImageResults(results []interface{}) (string, string) {
	finalResult := "pass"
	finalReason := ""
	for _, r := range results {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		label := stringFromMap(rm, "Label")
		suggestion := stringFromMap(rm, "Suggestion")

		rank := rankSuggestion(suggestion)
		currentRank := rankSuggestion(finalResult)
		if rank > currentRank {
			finalResult = toNormalizedResult(suggestion)
			finalReason = label
		}
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
