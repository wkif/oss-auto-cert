package qiniu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/nekoimi/oss-auto-cert/internal/config"
)

// Service 负责将证书部署到七牛 Fusion CDN 域名。
type Service struct {
	client  *Client
	domains []config.QiniuDomain
}

// NewService 创建七牛证书服务。未启用七牛时返回 nil。
func NewService(conf config.Qiniu) (*Service, error) {
	if !conf.Enabled {
		return nil, nil
	}
	if len(conf.Domains) == 0 {
		return nil, fmt.Errorf("七牛已启用但未配置 domains")
	}
	for _, d := range conf.Domains {
		if d.Domain == "" || strings.ContainsAny(d.Domain, "/: ?#*\\\t\n") {
			return nil, fmt.Errorf("七牛域名配置无效")
		}
	}
	client, err := NewClient(conf.AccessKey, conf.SecretKey)
	if err != nil {
		return nil, err
	}
	return &Service{client: client, domains: conf.Domains}, nil
}

// Deploy 上传证书并绑定到所有配置的七牛域名。
func (s *Service) Deploy(ctx context.Context, cert *certificate.Resource) error {
	if s == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if cert == nil {
		return fmt.Errorf("证书为空")
	}
	pair, err := tls.X509KeyPair(cert.Certificate, cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("证书与私钥校验失败: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	var targets []string
	for _, d := range s.domains {
		if leaf.VerifyHostname(d.Domain) == nil {
			targets = append(targets, d.Domain)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	certID, err := s.client.UploadCertificate(ctx, cert)
	if err != nil {
		return err
	}
	var failures []error
	for _, domain := range targets {
		if err := s.client.BindCertificate(ctx, domain, certID); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// DeployDomain 上传证书并绑定到指定七牛域名。
func (s *Service) DeployDomain(ctx context.Context, cert *certificate.Resource, domain string) error {
	if s == nil {
		return nil
	}
	if cert == nil {
		return fmt.Errorf("证书为空")
	}
	pair, err := tls.X509KeyPair(cert.Certificate, cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("证书与私钥校验失败: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return err
	}
	if err := leaf.VerifyHostname(domain); err != nil {
		return fmt.Errorf("证书不覆盖七牛域名(%s): %w", domain, err)
	}
	certID, err := s.client.UploadCertificate(ctx, cert)
	if err != nil {
		return err
	}
	return s.client.BindCertificate(ctx, domain, certID)
}

// Domains 返回配置的七牛域名。
func (s *Service) Domains() []string {
	if s == nil {
		return nil
	}
	result := make([]string, 0, len(s.domains))
	for _, domain := range s.domains {
		result = append(result, domain.Domain)
	}
	return result
}

// NeedsRenew 通过七牛管理 API 查询域名实际绑定证书的有效期。
func (s *Service) NeedsRenew(ctx context.Context, domain string, early time.Duration) (bool, time.Time, error) {
	var detail struct {
		Protocol string `json:"protocol"`
		HTTPS    struct {
			CertID string `json:"certId"`
		} `json:"https"`
	}
	if err := s.client.do(ctx, "qiniu", apiEndpoint, http.MethodGet, "/domain/"+url.PathEscape(domain), nil, &detail); err != nil {
		return false, time.Time{}, err
	}
	if detail.Protocol != "https" || detail.HTTPS.CertID == "" {
		return false, time.Time{}, fmt.Errorf("七牛域名(%s)未开启 HTTPS 或未绑定证书", domain)
	}
	var response struct {
		Cert struct {
			NotAfter int64 `json:"not_after"`
		} `json:"cert"`
	}
	if err := s.client.do(ctx, "qbox", fusionEndpoint, http.MethodGet, "/sslcert/"+url.PathEscape(detail.HTTPS.CertID), nil, &response); err != nil {
		return false, time.Time{}, err
	}
	if response.Cert.NotAfter <= 0 {
		return false, time.Time{}, fmt.Errorf("七牛域名(%s)证书有效期缺失", domain)
	}
	end := time.Unix(response.Cert.NotAfter, 0)
	return !end.After(time.Now().Add(early)), end, nil
}

// Reconcile 检查七牛域名线上证书，必要时申请并部署新证书。
func (s *Service) Reconcile(ctx context.Context, early time.Duration, obtain func(string) (*certificate.Resource, error), notify func(string)) {
	for _, domain := range s.Domains() {
		renew, end, err := s.NeedsRenew(ctx, domain, early)
		if err != nil {
			notify(fmt.Sprintf("七牛域名(%s)检查证书失败: %s", domain, err))
			continue
		}
		if !renew {
			notify(fmt.Sprintf("七牛域名(%s)未过期，过期日期: %s", domain, end.Format(time.RFC3339)))
			continue
		}
		notify(fmt.Sprintf("七牛域名(%s)证书即将过期，开始申请新证书", domain))
		cert, err := obtain(domain)
		if err != nil {
			notify(fmt.Sprintf("七牛域名(%s)申请证书失败: %s", domain, err))
			continue
		}
		if err := s.DeployDomain(ctx, cert, domain); err != nil {
			notify(fmt.Sprintf("七牛域名(%s)更新证书失败: %s", domain, err))
			continue
		}
		notify(fmt.Sprintf("七牛域名(%s)更新证书成功，请及时检查生效", domain))
	}
}
