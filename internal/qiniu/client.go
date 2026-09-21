package qiniu

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-acme/lego/v4/certificate"
)

const (
	apiEndpoint    = "https://api.qiniu.com"
	fusionEndpoint = "https://fusion.qiniuapi.com"
)

// Client 是七牛 Fusion API 客户端。
type Client struct {
	accessKey string
	secretKey string
	http      *http.Client
}

// NewClient 创建七牛 Fusion API 客户端。
func NewClient(accessKey, secretKey string) (*Client, error) {
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("七牛 AccessKey/SecretKey 未配置")
	}
	return &Client{accessKey: accessKey, secretKey: secretKey, http: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

func (c *Client) authorization(kind, method, host, path string, body []byte) string {
	data := []byte(path + "\n")
	if kind == "qiniu" {
		data = []byte(method + " " + path + "\nHost: " + host + contentTypeLine(body) + "\n\n" + string(body))
	}
	h := hmac.New(sha1.New, []byte(c.secretKey))
	_, _ = h.Write(data)
	sign := base64.URLEncoding.EncodeToString(h.Sum(nil))
	prefix := "QBox"
	if kind == "qiniu" {
		prefix = "Qiniu"
	}
	return prefix + " " + c.accessKey + ":" + sign
}

func (c *Client) do(ctx context.Context, kind, endpoint, method, requestPath string, payload any, result any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化七牛请求失败: %w", err)
		}
	}
	u, err := url.JoinPath(endpoint, requestPath)
	if err != nil {
		return fmt.Errorf("构造七牛请求地址失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建七牛请求失败: %w", err)
	}
	host := "fusion.qiniuapi.com"
	if kind == "qiniu" {
		host = "api.qiniu.com"
	}
	req.Header.Set("Authorization", c.authorization(kind, method, host, requestPath, body))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求七牛 API 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("读取七牛 API 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("七牛 API 返回 HTTP %d（请求 ID: %s）", resp.StatusCode, resp.Header.Get("X-Reqid"))
	}
	var status struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &status); err != nil {
			return fmt.Errorf("七牛返回无效 JSON: %w", err)
		}
		if (status.Code != 0 && status.Code != 200) || status.Error != "" {
			return fmt.Errorf("七牛业务请求失败，code=%d", status.Code)
		}
	}
	if result != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("解析七牛 API 响应失败: %w", err)
		}
	}
	return nil
}

type uploadCertificateRequest struct {
	Name       string `json:"name"`
	PrivateKey string `json:"pri"`
	Chain      string `json:"ca"`
}

type uploadCertificateResponse struct {
	CertID string `json:"certID"`
}

// UploadCertificate 上传 ACME 证书并返回七牛证书 ID。
func (c *Client) UploadCertificate(ctx context.Context, cert *certificate.Resource) (string, error) {
	name := "oss-auto-cert-" + cert.Domain + "-" + time.Now().UTC().Format("20060102150405")
	request := uploadCertificateRequest{
		Name:       name,
		PrivateKey: string(cert.PrivateKey),
		Chain:      string(cert.Certificate) + string(cert.IssuerCertificate),
	}
	var response uploadCertificateResponse
	if err := c.do(ctx, "qbox", fusionEndpoint, http.MethodPost, "/sslcert", request, &response); err != nil {
		return "", fmt.Errorf("上传七牛证书失败: %w", err)
	}
	if response.CertID != "" {
		return response.CertID, nil
	}
	return "", fmt.Errorf("七牛上传证书响应缺少证书 ID")
}

// BindCertificate 更新已开启 HTTPS 的域名证书，保留现有 HTTPS 配置。
func (c *Client) BindCertificate(ctx context.Context, domain, certID string) error {
	path := "/domain/" + url.PathEscape(domain)
	var detail struct {
		Protocol string                     `json:"protocol"`
		HTTPS    map[string]json.RawMessage `json:"https"`
	}
	if err := c.do(ctx, "qiniu", apiEndpoint, http.MethodGet, path, nil, &detail); err != nil {
		return err
	}
	if detail.Protocol != "https" || detail.HTTPS == nil {
		return fmt.Errorf("七牛域名(%s)未开启 HTTPS，需先在控制台配置", domain)
	}
	// 只替换证书 ID，不改变跳转、HTTP/2、TLS 等现有设置。
	id, err := json.Marshal(certID)
	if err != nil {
		return err
	}
	detail.HTTPS["certId"] = id
	if err := c.do(ctx, "qiniu", apiEndpoint, http.MethodPut, path+"/httpsconf", detail.HTTPS, nil); err != nil {
		return fmt.Errorf("绑定七牛域名(%s)证书失败: %w", domain, err)
	}
	return nil
}

func contentTypeLine(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	return "\nContent-Type: application/json"
}
