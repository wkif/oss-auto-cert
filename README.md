# 阿里云 OSS 证书自动化工具

[![GitHub Release](https://img.shields.io/github/release/nekoimi/oss-auto-cert.svg)](https://github.com/nekoimi/oss-auto-cert/releases)
[![License](https://img.shields.io/github/license/nekoimi/oss-auto-cert.svg)](LICENSE)

基于 Let's Encrypt 实现阿里云 OSS/CDN SSL 证书自动续期和管理。

## 功能特性

- **自动检测** - 定时检测 OSS 自定义域名证书过期情况
- **自动续期** - 基于 Let's Encrypt 自动申请/续期 SSL 证书
- **自动部署** - 自动上传并更新阿里云 OSS、CDN 和 CAS 证书
- **消息通知** - 支持企业微信、钉钉、飞书等 Webhook 通知
- **多架构支持** - 支持 linux/amd64 和 linux/arm64

## 架构原理

借助 Let's Encrypt 证书，使用阿里云 OSS API、证书管理服务 API (CAS) 和 CDN API 实现 OSS 自定义域名证书的自动更新。

![oss-auto-cert.png](oss-auto-cert.png)

**证书更新目标资源：**
- 阿里云 OSS 对象存储
- 阿里云 CDN 加速域名
- 阿里云证书管理服务 (CAS)

## 快速开始

### 前置要求

- 阿里云 Access Key ID 和 Access Key Secret
- 具有下列权限的阿里云 RAM 账号：
  - `AliyunOSSFullAccess` - OSS 管理权限
  - `AliyunCDNFullAccess` - CDN 管理权限
  - `AliyunCASFullAccess` - 证书管理服务权限

### 1. 准备环境变量

```bash
export OSS_ACCESS_KEY_ID="your-access-key-id"
export OSS_ACCESS_KEY_SECRET="your-access-key-secret"
```

### 2. 创建配置文件

创建 `config.yaml`：

```yaml
# 通知 Webhook（可选）
webhook: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxx

# 证书配置
acme:
  email: your-email@example.com
  expired-early: 30

# OSS Bucket 列表
buckets:
  - name: my-bucket
    endpoint: oss-cn-hangzhou.aliyuncs.com
```

### 3. 运行

**Docker（推荐）：**

```bash
docker run -d --rm \
  -v $PWD/config.yaml:/etc/oss-auto-cert/config.yaml \
  -e OSS_ACCESS_KEY_ID=$OSS_ACCESS_KEY_ID \
  -e OSS_ACCESS_KEY_SECRET=$OSS_ACCESS_KEY_SECRET \
  ghcr.io/nekoimi/oss-auto-cert:latest
```

**或 Docker Compose：**

```yaml
version: "3.8"
services:
  oss-auto-cert:
    image: ghcr.io/nekoimi/oss-auto-cert:latest
    volumes:
      - ./config.yaml:/etc/oss-auto-cert/config.yaml
    environment:
      OSS_ACCESS_KEY_ID: ${OSS_ACCESS_KEY_ID}
      OSS_ACCESS_KEY_SECRET: ${OSS_ACCESS_KEY_SECRET}
    restart: unless-stopped
```

```bash
docker-compose up -d
```

## 配置说明

### 完整配置示例

```yaml
# 消息通知 Webhook 地址
webhook: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxx

# Webhook 请求体模板（可选，默认企业微信格式）
webhook-tpl: |
  {
    "msgtype": "text",
    "text": {
      "content": "{{ .Message }}"
    }
  }

# ACME/Let's Encrypt 配置
acme:
  email: admin@example.com              # 申请证书邮箱（必填）
  data-dir: /var/lib/oss-auto-cert     # 证书保存目录
  expired-early: 30                     # 提前续期天数（默认15）

# OSS Bucket 配置列表
buckets:
  - name: bucket-name-1
    endpoint: oss-cn-hangzhou.aliyuncs.com
  - name: bucket-name-2
    endpoint: oss-cn-beijing.aliyuncs.com

# 七牛 Fusion CDN（可选）
qiniu:
  enabled: true
  domains:
    - domain: cdn.example.com
```

### 环境变量

| 变量名 | 说明 | 必需 |
|--------|------|:----:|
| `OSS_ACCESS_KEY_ID` | 阿里云 Access Key ID | ✅ |
| `OSS_ACCESS_KEY_SECRET` | 阿里云 Access Key Secret | ✅ |
| `ACME_EMAIL` | 证书申请邮箱 | ❌ |
| `ACME_DATA_DIR` | 证书存储目录 | ❌ |
| `ACME_EXPIRED_EARLY` | 提前续期天数 | ❌ |
| `DEBUG` | 调试模式（true/false） | ❌ |
| `QINIU_ACCESS_KEY` | 七牛 AccessKey（启用七牛时必需） | ❌ |
| `QINIU_SECRET_KEY` | 七牛 SecretKey（启用七牛时必需） | ❌ |

**说明：** 环境变量优先级高于配置文件。

### Webhook 通知

支持企业微信、钉钉、飞书等，详见 [使用文档](docs/usage.md#webhook-通知配置)。

## 部署方式

### Docker 镜像

| 镜像源 | 地址 |
|--------|------|
| Docker Hub | `nekoimi/oss-auto-cert:latest` |
| GitHub Container Registry | `ghcr.io/nekoimi/oss-auto-cert:latest` |
| 阿里云（国内） | `registry.cn-hangzhou.aliyuncs.com/nekoimi/oss-auto-cert:latest` |

**持久化证书：**

```bash
docker run -d --rm \
  -v $PWD/config.yaml:/etc/oss-auto-cert/config.yaml \
  -v $PWD/certs:/var/lib/oss-auto-cert \
  -e OSS_ACCESS_KEY_ID=xxx \
  -e OSS_ACCESS_KEY_SECRET=xxx \
  ghcr.io/nekoimi/oss-auto-cert:latest
```

### Systemd 部署

详见 [使用文档](docs/usage.md#systemd-部署)。

## 工作原理

1. **定时检测** - 每 6 小时检测所有配置的 Bucket
2. **过期判断** - 根据配置判断证书是否需要续期
3. **证书申请** - 使用 ACME/Let's Encrypt 申请新证书
4. **上传部署** - 上传到阿里云 CAS 并更新 OSS/CDN
5. **消息通知** - 通过 Webhook 发送处理结果

## 详细文档

- [完整使用文档](docs/usage.md)
- [开发规范](AGENTS.md)

## 开源协议

[LICENSE](LICENSE)

## 相关项目

- [go-acme/lego](https://github.com/go-acme/lego) - Let's Encrypt ACME 客户端
- [阿里云 OpenAPI](https://api.aliyun.com)

## Star History

[![Star History Chart](https://api.star-history.com/image?repos=nekoimi/oss-auto-cert&type=date&legend=top-left)](https://www.star-history.com/?repos=nekoimi%2Foss-auto-cert&type=date&legend=top-left)
