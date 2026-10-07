package mailstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// S3Config configures an S3-compatible object store (AWS, MinIO, etc.).
type S3Config struct {
	Endpoint  string // e.g. https://s3.amazonaws.com or http://127.0.0.1:9000
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Prefix    string // optional key prefix, e.g. "maildir/"
	PathStyle bool   // true for MinIO / path-style; false for virtual-hosted
}

// S3Blob implements Blob via the S3 REST API (SigV4).
type S3Blob struct {
	cfg    S3Config
	client *http.Client
}

func NewS3Blob(cfg S3Config) (*S3Blob, error) {
	cfg.Endpoint = strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3: endpoint, bucket, access_key, and secret_key are required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Prefix != "" && !strings.HasSuffix(cfg.Prefix, "/") {
		cfg.Prefix += "/"
	}
	return &S3Blob{
		cfg:    cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (b *S3Blob) key(k string) string {
	k = strings.TrimPrefix(k, "/")
	return b.cfg.Prefix + k
}

func (b *S3Blob) Put(ctx context.Context, key string, data []byte) error {
	req, err := b.newReq(ctx, http.MethodPut, b.key(key), data, "application/octet-stream")
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("s3 put %s: %s: %s", key, res.Status, bytes.TrimSpace(body))
	}
	return nil
}

func (b *S3Blob) Get(ctx context.Context, key string) ([]byte, error) {
	req, err := b.newReq(ctx, http.MethodGet, b.key(key), nil, "")
	if err != nil {
		return nil, err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errBlobNotFound
	}
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("s3 get %s: %s: %s", key, res.Status, bytes.TrimSpace(body))
	}
	return io.ReadAll(res.Body)
}

func (b *S3Blob) Delete(ctx context.Context, key string) error {
	req, err := b.newReq(ctx, http.MethodDelete, b.key(key), nil, "")
	if err != nil {
		return err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || res.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	return fmt.Errorf("s3 delete %s: %s: %s", key, res.Status, bytes.TrimSpace(body))
}

func (b *S3Blob) newReq(ctx context.Context, method, objectKey string, body []byte, contentType string) (*http.Request, error) {
	u, err := b.objectURL(objectKey)
	if err != nil {
		return nil, err
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rdr)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	now := time.Now().UTC()
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	if int64(len(body)) > 0 || method == http.MethodPut {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}
	if err := signV4(req, b.cfg, payloadHash, now); err != nil {
		return nil, err
	}
	return req, nil
}

func (b *S3Blob) objectURL(objectKey string) (*url.URL, error) {
	base, err := url.Parse(b.cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	objectKey = strings.TrimPrefix(objectKey, "/")
	if b.cfg.PathStyle {
		base.Path = "/" + b.cfg.Bucket + "/" + objectKey
		return base, nil
	}
	// virtual-hosted-style: bucket.endpoint/key
	base.Host = b.cfg.Bucket + "." + base.Host
	base.Path = "/" + objectKey
	return base, nil
}

func sha256Hex(data []byte) string {
	if data == nil {
		data = []byte{}
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func signV4(req *http.Request, cfg S3Config, payloadHash string, now time.Time) error {
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	if req.Header.Get("Content-Type") != "" {
		signedHeaders = "content-type;host;x-amz-content-sha256;x-amz-date"
	}

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.Query().Encode()

	var canonicalHeaders strings.Builder
	host := req.URL.Host
	if req.Header.Get("Content-Type") != "" {
		canonicalHeaders.WriteString("content-type:" + strings.TrimSpace(req.Header.Get("Content-Type")) + "\n")
	}
	canonicalHeaders.WriteString("host:" + host + "\n")
	canonicalHeaders.WriteString("x-amz-content-sha256:" + payloadHash + "\n")
	canonicalHeaders.WriteString("x-amz-date:" + amzDate + "\n")

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credScope := dateStamp + "/" + cfg.Region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(cfg.SecretKey, dateStamp, cfg.Region, "s3")
	sig := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	auth := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		cfg.AccessKey, credScope, signedHeaders, sig,
	)
	req.Header.Set("Authorization", auth)
	return nil
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(data))
	return m.Sum(nil)
}
