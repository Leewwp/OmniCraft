package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
)

var ErrOSSNotConfigured = errors.New("oss config is incomplete")

type UploadValidationError struct {
	Message string
}

func (e *UploadValidationError) Error() string {
	return e.Message
}

type PresignUploadRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileType    string `json:"file_type" binding:"required"`
	MimeType    string `json:"mime_type" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required"`
	DurationSec *int   `json:"duration_sec"`
}

type PresignUploadResponse struct {
	UploadURL string `json:"upload_url"`
	OSSKey    string `json:"oss_key"`
	ExpiresIn int64  `json:"expires_in"`
	GrantID   string `json:"grant_id"`
}

type OSSService struct {
	cfg    *config.Config
	client *aliyun.OSSClient
}

func NewOSSService(cfg *config.Config) (*OSSService, error) {
	if cfg == nil {
		return nil, ErrOSSNotConfigured
	}
	if strings.TrimSpace(cfg.OSS.Endpoint) == "" || strings.TrimSpace(cfg.OSS.AccessKeyID) == "" || strings.TrimSpace(cfg.OSS.AccessKeySecret) == "" || strings.TrimSpace(cfg.OSS.BucketName) == "" {
		return nil, ErrOSSNotConfigured
	}

	client, err := aliyun.NewOSSClient(cfg.OSS.Endpoint, cfg.OSS.AccessKeyID, cfg.OSS.AccessKeySecret, cfg.OSS.BucketName)
	if err != nil {
		return nil, err
	}

	return &OSSService{cfg: cfg, client: client}, nil
}

func (s *OSSService) GeneratePresignUploadURL(ctx context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error) {
	_ = ctx
	if s == nil || s.client == nil {
		return nil, ErrOSSNotConfigured
	}

	normalizedFileType := strings.ToLower(strings.TrimSpace(req.FileType))
	normalizedMime := strings.ToLower(strings.TrimSpace(req.MimeType))
	ext := strings.ToLower(filepath.Ext(req.FileName))

	if req.FileSize <= 0 {
		return nil, &UploadValidationError{Message: "file_size must be greater than 0"}
	}

	if err := s.validateUploadByType(normalizedFileType, normalizedMime, req.FileSize, req.DurationSec, ext); err != nil {
		return nil, err
	}

	ossKey := s.buildOSSKey(normalizedFileType, ext, userID)
	const signedTTL = 15 * time.Minute
	url, err := s.client.GetSignedURL(ossKey, http.MethodPut, signedTTL, oss.ContentType(normalizedMime))
	if err != nil {
		return nil, err
	}

	return &PresignUploadResponse{
		UploadURL: url,
		OSSKey:    ossKey,
		ExpiresIn: int64(signedTTL.Seconds()),
	}, nil
}

func (s *OSSService) GenerateFeedbackPresignUploadURL(ctx context.Context, req PresignUploadRequest, userID int64) (*PresignUploadResponse, error) {
	_ = ctx
	if s == nil || s.client == nil {
		return nil, ErrOSSNotConfigured
	}

	normalizedMime := strings.ToLower(strings.TrimSpace(req.MimeType))
	ext := strings.ToLower(filepath.Ext(req.FileName))

	if req.FileSize <= 0 {
		return nil, &UploadValidationError{Message: "file_size must be greater than 0"}
	}
	if err := s.validateUploadByType("image", normalizedMime, req.FileSize, req.DurationSec, ext); err != nil {
		return nil, err
	}

	ossKey := s.buildOSSKey("feedback", ext, userID)
	const signedTTL = 15 * time.Minute
	url, err := s.client.GetSignedURL(ossKey, http.MethodPut, signedTTL, oss.ContentType(normalizedMime))
	if err != nil {
		return nil, err
	}

	return &PresignUploadResponse{
		UploadURL: url,
		OSSKey:    ossKey,
		ExpiresIn: int64(signedTTL.Seconds()),
	}, nil
}

func (s *OSSService) GeneratePresignDownloadURL(ctx context.Context, ossKey string, ttl time.Duration) (string, error) {
	_ = ctx
	if s == nil || s.client == nil {
		return "", ErrOSSNotConfigured
	}
	if strings.TrimSpace(ossKey) == "" {
		return "", &UploadValidationError{Message: "oss_key is required"}
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return s.client.GetSignedURL(strings.TrimSpace(ossKey), http.MethodGet, ttl)
}

func (s *OSSService) VerifyUploadedObject(ctx context.Context, grant UploadGrant) error {
	_ = ctx
	if s == nil || s.client == nil {
		return ErrOSSNotConfigured
	}
	meta, err := s.client.GetObjectMeta(grant.OSSKey)
	if err != nil {
		return err
	}
	if meta.ContentLength != grant.FileSize {
		return &UploadValidationError{Message: "uploaded file size does not match grant"}
	}
	if !strings.EqualFold(strings.TrimSpace(meta.ContentType), strings.TrimSpace(grant.MimeType)) {
		return &UploadValidationError{Message: "uploaded content type does not match grant"}
	}
	return nil
}

func (s *OSSService) validateUploadByType(fileType, mimeType string, fileSize int64, durationSec *int, ext string) error {
	limitMB := 0
	switch fileType {
	case "video":
		limitMB = s.cfg.Limits.VideoMaxMB
		if !strings.HasPrefix(mimeType, "video/") {
			return &UploadValidationError{Message: "mime_type must be video/*"}
		}
		if durationSec != nil && *durationSec > s.cfg.Limits.VideoMaxSec {
			return &UploadValidationError{Message: fmt.Sprintf("video duration exceeds %d seconds", s.cfg.Limits.VideoMaxSec)}
		}
	case "image":
		limitMB = s.cfg.Limits.ImageMaxMB
		if !strings.HasPrefix(mimeType, "image/") {
			return &UploadValidationError{Message: "mime_type must be image/*"}
		}
	case "text":
		limitMB = s.cfg.Limits.TextMaxMB
		if !(strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf") {
			return &UploadValidationError{Message: "unsupported text mime_type"}
		}
	case "mod":
		limitMB = s.cfg.Limits.ModMaxMB
		if mimeType != "application/zip" && mimeType != "application/x-zip-compressed" {
			return &UploadValidationError{Message: "mod must be a zip package"}
		}
	case "sheet_music":
		limitMB = s.cfg.Limits.SheetMusicMaxMB
		if !s.isAllowedSheetMusicExt(ext) {
			return &UploadValidationError{Message: "unsupported sheet_music extension"}
		}
	default:
		return &UploadValidationError{Message: "unsupported file_type"}
	}

	limitBytes := int64(limitMB) * 1024 * 1024
	if fileSize > limitBytes {
		return &UploadValidationError{Message: fmt.Sprintf("file size exceeds %dMB", limitMB)}
	}

	return nil
}

func (s *OSSService) GenerateVideoSnapshotURL(ctx context.Context, ossKey string) (string, error) {
	_ = ctx
	if s == nil || s.client == nil {
		return "", ErrOSSNotConfigured
	}
	if strings.TrimSpace(ossKey) == "" {
		return "", &UploadValidationError{Message: "oss_key is required"}
	}
	const coverExpiry = 7 * 24 * time.Hour
	return s.client.GetVideoSnapshotURL(strings.TrimSpace(ossKey), coverExpiry, 480)
}

// PersistentObjectURL derives a stable URL only through the explicitly
// configured delivery domain. Private OSS buckets must be read through the
// signed download endpoint; never synthesize an unsigned bucket URL.
func (s *OSSService) PersistentObjectURL(ossKey string) string {
	if s == nil || s.cfg == nil {
		return ""
	}
	key := strings.TrimSpace(ossKey)
	if key == "" {
		return ""
	}
	if domain := strings.TrimSpace(s.cfg.OSS.Domain); domain != "" {
		return strings.TrimSuffix(domain, "/") + "/" + key
	}
	return ""
}

func (s *OSSService) isAllowedSheetMusicExt(ext string) bool {
	for _, allowed := range s.cfg.Upload.SheetMusicExtensions {
		if strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(ext)) {
			return true
		}
	}
	return false
}

func (s *OSSService) buildOSSKey(fileType, ext string, userID int64) string {
	stamp := time.Now().UTC()
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("uploads/%d/%s/%d/%02d/%02d/%d%s", userID, fileType, stamp.Year(), stamp.Month(), stamp.Day(), stamp.UnixNano(), ext)
	}
	randomPart := hex.EncodeToString(raw)
	return fmt.Sprintf("uploads/%d/%s/%d/%02d/%02d/%d_%s%s", userID, fileType, stamp.Year(), stamp.Month(), stamp.Day(), stamp.Unix(), randomPart, ext)
}
