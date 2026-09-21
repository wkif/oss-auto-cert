package config

import (
	"io"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v3"
)

const (
	DefaultExpiredEarly = 15
	DefaultConfigPath   = "/etc/oss-auto-cert/config.yaml"
)

var (
	// expiredEarlyDay 提前过期时间点 默认15天
	expiredEarlyDay = DefaultExpiredEarly
	// expiredEarlyTime 提前过期时间
	expiredEarlyTime = time.Hour * 24 * DefaultExpiredEarly
)

type Config struct {
	// 配置文件路径
	// 默认路径: DefaultConfigPath
	Path string
	// 通知地址
	Webhook string `yaml:"webhook"`
	// 通知消息模版
	WebhookTpl string `yaml:"webhook-tpl"`
	// Acme配置
	Acme Acme `yaml:"acme"`
	// Bucket配置
	Buckets []Bucket `yaml:"buckets"`
	// 七牛 Fusion CDN 配置
	Qiniu Qiniu `yaml:"qiniu"`
}

type Acme struct {
	// 证书申请邮箱
	Email string `yaml:"email"`

	// 证书保存位置
	DataDir string `yaml:"data-dir"`

	// 证书提前renew时间
	ExpiredEarly int `yaml:"expired-early"`
}

// Bucket OSS存储Bucket配置
type Bucket struct {
	// bucket名称
	Name string `yaml:"name"`
	// Endpoint
	Endpoint string `yaml:"endpoint"`
}

// Qiniu 保存七牛 Fusion CDN 配置。
type Qiniu struct {
	// 是否启用七牛证书部署。
	Enabled bool `yaml:"enabled"`
	// 七牛 AccessKey。未配置时从 QINIU_ACCESS_KEY 读取。
	AccessKey string `yaml:"access-key"`
	// 七牛 SecretKey。未配置时从 QINIU_SECRET_KEY 读取。
	SecretKey string `yaml:"secret-key"`
	// 需要部署证书的 Fusion CDN 域名。
	Domains []QiniuDomain `yaml:"domains"`
}

// QiniuDomain 是七牛 Fusion CDN 域名配置。
type QiniuDomain struct {
	Domain string `yaml:"domain"`
}

// LoadOptions 加载配置
func (conf *Config) LoadOptions() {
	if conf.Path == "" {
		conf.Path = DefaultConfigPath
	}

	f, err := os.Open(conf.Path)
	if err != nil {
		log.Fatalf("读取配置文件 %s 出错: %s", conf.Path, err.Error())
	}
	defer f.Close()

	bts, err := io.ReadAll(f)
	if err != nil {
		log.Fatalf("读取配置文件 %s 出错: %s", conf.Path, err.Error())
	}

	err = yaml.Unmarshal(bts, &conf)
	if err != nil {
		log.Fatalf("读取配置文件 %s 出错: %s", conf.Path, err.Error())
	}

	conf.setExpiredEarlyTime()

	log.Debugf("已加载配置，Bucket 数量: %d", len(conf.Buckets))
}

func (conf *Config) LoadOptionsFromEnv() {
	if value := os.Getenv("QINIU_ACCESS_KEY"); value != "" {
		conf.Qiniu.AccessKey = value
	}
	if value := os.Getenv("QINIU_SECRET_KEY"); value != "" {
		conf.Qiniu.SecretKey = value
	}
	value := os.Getenv("ACME_EMAIL")
	if value != "" {
		conf.Acme.Email = value
		log.Debugf("set acme (email) from env: %s", value)
	}

	value = os.Getenv("ACME_DATA_DIR")
	if value != "" {
		conf.Acme.DataDir = value
		log.Debugf("set acme (data dir) from env: %s", value)
	}

	value = os.Getenv("ACME_EXPIRED_EARLY")
	if value != "" {
		if valueInt, err := strconv.Atoi(value); err != nil {
			log.Warnf("环境变量 ACME_EXPIRED_EARLY 无效: %s", err)
		} else {
			conf.Acme.ExpiredEarly = valueInt
			log.Debugf("set acme (expired early) from env: %d", valueInt)
			conf.setExpiredEarlyTime()
		}
	}

	log.Debugf("已加载配置，Bucket 数量: %d", len(conf.Buckets))
}

func (conf *Config) setExpiredEarlyTime() {
	// 根据配置更新证书更新提前过期时间
	expiredEarlyDay = max(DefaultExpiredEarly, conf.Acme.ExpiredEarly)
	expiredEarlyTime = time.Hour * 24 * time.Duration(expiredEarlyDay)
}

func GetExpiredEarlyDay() int {
	return expiredEarlyDay
}

func GetExpiredEarlyTime() time.Duration {
	return expiredEarlyTime
}
