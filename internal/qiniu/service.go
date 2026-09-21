package qiniu

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

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
