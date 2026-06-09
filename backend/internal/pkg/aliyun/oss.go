package aliyun

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/sts"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type STSToken struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"-"`
	SecurityToken   string `json:"security_token"`
	Expiration      string `json:"expiration"`
}

type OSSClient struct {
	client          *oss.Client
	bucket          *oss.Bucket
	accessKeyID     string
	accessKeySecret string
}

type ObjectMeta struct {
	ContentLength int64
	ContentType   string
}

func NewOSSClient(endpoint, accessKeyID, accessKeySecret, bucketName string) (*OSSClient, error) {
	if strings.TrimSpace(endpoint) == "" || strings.TrimSpace(accessKeyID) == "" || strings.TrimSpace(accessKeySecret) == "" || strings.TrimSpace(bucketName) == "" {
		return nil, fmt.Errorf("oss config is incomplete")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}

	return &OSSClient{
		client:          client,
		bucket:          bucket,
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
	}, nil
}

func (c *OSSClient) PutObject(ossKey string, reader io.Reader, options ...oss.Option) error {
	return c.bucket.PutObject(ossKey, reader, options...)
}

func (c *OSSClient) DeleteObject(ossKey string) error {
	return c.bucket.DeleteObject(ossKey)
}

func (c *OSSClient) GetSignedURL(ossKey, method string, expires time.Duration, options ...oss.Option) (string, error) {
	expiresSec := int64(expires.Seconds())
	if expiresSec <= 0 {
		expiresSec = 900
	}

	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPut:
		return c.bucket.SignURL(ossKey, oss.HTTPPut, expiresSec, options...)
	case http.MethodGet:
		return c.bucket.SignURL(ossKey, oss.HTTPGet, expiresSec, options...)
	default:
		return "", fmt.Errorf("unsupported sign method: %s", method)
	}
}

func (c *OSSClient) GetObjectMeta(ossKey string) (*ObjectMeta, error) {
	props, err := c.bucket.GetObjectDetailedMeta(ossKey)
	if err != nil {
		return nil, err
	}
	length, _ := strconv.ParseInt(props.Get("Content-Length"), 10, 64)
	return &ObjectMeta{
		ContentLength: length,
		ContentType:   props.Get("Content-Type"),
	}, nil
}

func (c *OSSClient) GetVideoSnapshotURL(ossKey string, expires time.Duration, width int) (string, error) {
	expiresSec := int64(expires.Seconds())
	if expiresSec <= 0 {
		expiresSec = 3600
	}
	if width <= 0 {
		width = 480
	}
	process := fmt.Sprintf("video/snapshot,t_0,f_jpg,w_%d", width)
	return c.bucket.SignURL(ossKey, oss.HTTPGet, expiresSec, oss.Process(process))
}

func (c *OSSClient) GetSTS(regionID, roleArn, sessionName string, durationSeconds int64) (*STSToken, error) {
	if strings.TrimSpace(regionID) == "" || strings.TrimSpace(roleArn) == "" {
		return nil, fmt.Errorf("region and role arn are required")
	}
	if strings.TrimSpace(sessionName) == "" {
		sessionName = "omnicraft-upload"
	}
	if durationSeconds <= 0 {
		durationSeconds = 3600
	}

	stsClient, err := sts.NewClientWithAccessKey(regionID, c.accessKeyID, c.accessKeySecret)
	if err != nil {
		return nil, err
	}

	req := sts.CreateAssumeRoleRequest()
	req.Scheme = "https"
	req.RoleArn = roleArn
	req.RoleSessionName = sessionName
	req.DurationSeconds = requests.Integer(strconv.FormatInt(durationSeconds, 10))

	resp, err := stsClient.AssumeRole(req)
	if err != nil {
		return nil, err
	}

	return &STSToken{
		AccessKeyID:     resp.Credentials.AccessKeyId,
		AccessKeySecret: resp.Credentials.AccessKeySecret,
		SecurityToken:   resp.Credentials.SecurityToken,
		Expiration:      resp.Credentials.Expiration,
	}, nil
}
