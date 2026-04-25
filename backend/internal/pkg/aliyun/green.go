package aliyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	greenapi "github.com/aliyun/alibaba-cloud-sdk-go/services/green"
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

	payload := map[string]interface{}{
		"scenes": []string{"antispam"},
		"tasks": []map[string]string{
			{
				"dataId":  c.newDataID("text"),
				"content": text,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := greenapi.CreateTextScanRequest()
	req.SetContent(body)
	req.SetContentType("application/json")

	resp, err := client.TextScan(req)
	if err != nil {
		return nil, err
	}

	return parseGreenScanResponse("text", resp.GetHttpContentString())
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

	payload := map[string]interface{}{
		"scenes": []string{"porn", "terrorism", "ad"},
		"tasks": []map[string]string{
			{
				"dataId": c.newDataID("image"),
				"url":    imageURL,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := greenapi.CreateImageSyncScanRequest()
	req.SetContent(body)
	req.SetContentType("application/json")

	resp, err := client.ImageSyncScan(req)
	if err != nil {
		return nil, err
	}

	return parseGreenScanResponse("image", resp.GetHttpContentString())
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

	task := map[string]interface{}{
		"dataId": c.newDataID("video"),
		"url":    videoURL,
	}
	if strings.TrimSpace(callbackURL) != "" {
		task["callback"] = callbackURL
	}

	payload := map[string]interface{}{
		"scenes": []string{"porn", "terrorism", "ad"},
		"tasks":  []map[string]interface{}{task},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := greenapi.CreateVideoAsyncScanRequest()
	req.SetContent(body)
	req.SetContentType("application/json")

	resp, err := client.VideoAsyncScan(req)
	if err != nil {
		return nil, err
	}

	rawBody := resp.GetHttpContentString()
	parsed, parseErr := parseGreenRaw(rawBody)
	if parseErr != nil {
		return nil, parseErr
	}

	result := &GreenScanResult{
		Result:      "review",
		Reason:      "video_async_submitted",
		RawResponse: parsed,
	}
	if taskID := extractTaskID(parsed); taskID != "" {
		result.TaskID = taskID
	}

	return result, nil
}

func (c *GreenClient) newClient() (*greenapi.Client, error) {
	client, err := greenapi.NewClientWithAccessKey(c.region, c.accessKeyID, c.accessKeySecret)
	if err != nil {
		return nil, err
	}
	greenapi.SetEndpointDataToClient(client)
	return client, nil
}

func (c *GreenClient) newDataID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func parseGreenScanResponse(contentType, rawBody string) (*GreenScanResult, error) {
	parsed, err := parseGreenRaw(rawBody)
	if err != nil {
		return nil, err
	}

	suggestion, label := extractSuggestion(parsed)
	result := &GreenScanResult{
		Result:      toNormalizedResult(suggestion),
		Reason:      label,
		RawResponse: parsed,
	}
	if result.Reason == "" {
		result.Reason = contentType + "_scan"
	}

	return result, nil
}

func parseGreenRaw(rawBody string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(rawBody)
	if trimmed == "" {
		return nil, errors.New("green api returned empty response body")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, err
	}

	return parsed, nil
}

func extractSuggestion(raw map[string]interface{}) (suggestion string, label string) {
	dataSlice, ok := raw["data"].([]interface{})
	if !ok {
		return "review", ""
	}

	finalSuggestion := "pass"
	finalLabel := ""
	for _, entry := range dataSlice {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		resultsSlice, ok := entryMap["results"].([]interface{})
		if !ok {
			continue
		}
		for _, r := range resultsSlice {
			resultMap, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			current := toStringValue(resultMap["suggestion"])
			if rankSuggestion(current) > rankSuggestion(finalSuggestion) {
				finalSuggestion = current
				finalLabel = toStringValue(resultMap["label"])
			}
		}
	}

	if finalSuggestion == "" {
		return "review", finalLabel
	}
	return finalSuggestion, finalLabel
}

func extractTaskID(raw map[string]interface{}) string {
	dataSlice, ok := raw["data"].([]interface{})
	if !ok {
		return ""
	}

	for _, entry := range dataSlice {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		taskID := toStringValue(entryMap["taskId"])
		if taskID != "" {
			return taskID
		}
	}

	return ""
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

func toStringValue(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
