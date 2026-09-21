package qiniu

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-acme/lego/v4/certificate"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestSignatures(t *testing.T) {
	c, err := NewClient("ak", "sk")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ kind, method, host, path, body, data, prefix string }{
		{"qbox", "POST", "fusion.qiniuapi.com", "/sslcert", `{"ca":"secret"}`, "/sslcert\n", "QBox"},
		{"qbox", "GET", "fusion.qiniuapi.com", "/sslcert?limit=10", "", "/sslcert?limit=10\n", "QBox"},
		{"qiniu", "PUT", "api.qiniu.com", "/domain/test.example.com/httpsconf", `{}`, "PUT /domain/test.example.com/httpsconf\nHost: api.qiniu.com\nContent-Type: application/json\n\n{}", "Qiniu"},
		{"qiniu", "GET", "api.qiniu.com", "/domain/test.example.com", "", "GET /domain/test.example.com\nHost: api.qiniu.com\n\n", "Qiniu"},
	} {
		h := hmac.New(sha1.New, []byte("sk"))
		if _, err := h.Write([]byte(tc.data)); err != nil {
			t.Fatal(err)
		}
		want := tc.prefix + " ak:" + base64.URLEncoding.EncodeToString(h.Sum(nil))
		if got := c.authorization(tc.kind, tc.method, tc.host, tc.path, []byte(tc.body)); got != want {
			t.Fatalf("签名错误: %s != %s", got, want)
		}
	}
}
func TestUploadAndBind(t *testing.T) {
	c, err := NewClient("ak", "sk")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			if r.URL.Host != "fusion.qiniuapi.com" || r.URL.Path != "/sslcert" || r.Method != "POST" {
				t.Fatal(r.URL, r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["pri"] != "key" || body["ca"] != "full-chainalready-in-chain" || len(body) != 3 {
				t.Fatal("错误的上传字段", body)
			}
			return response(`{"certID":"new-id"}`), nil
		case 2:
			return response(`{"protocol":"https","https":{"certId":"old","forceHttps":false,"http2Enable":false,"tlsVersions":"TLSv1.2"}}`), nil
		case 3:
			if r.Method != "PUT" || !strings.HasPrefix(r.Header.Get("Authorization"), "Qiniu ") {
				t.Fatal(r.Method, r.Header)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["certId"] != "new-id" || body["forceHttps"] != false || body["http2Enable"] != false || body["tlsVersions"] != "TLSv1.2" {
				t.Fatal(body)
			}
			return response(`{"code":200}`), nil
		default:
			t.Fatal("多余请求")
			return nil, nil
		}
	})
	id, err := c.UploadCertificate(context.Background(), &certificate.Resource{Domain: "test.example.com", Certificate: []byte("full-chain"), PrivateKey: []byte("key"), IssuerCertificate: []byte("already-in-chain")})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.BindCertificate(context.Background(), "test.example.com", id); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}
func TestBusinessError(t *testing.T) {
	c, err := NewClient("ak", "sk")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return response(`{"code":400324,"error":"private-key-secret"}`), nil
	})
	_, err = c.UploadCertificate(context.Background(), &certificate.Resource{})
	if err == nil || strings.Contains(err.Error(), "private-key-secret") {
		t.Fatalf("未正确处理/脱敏业务错误: %v", err)
	}
}
