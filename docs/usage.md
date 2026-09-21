# 阿里云 OSS 证书自动化工具使用文档

## 简介

oss-auto-cert 是一个基于 Let's Encrypt 实现阿里云 OSS/CDN SSL 证书自动管理的工具。它能够自动检测证书过期并续期，同时更新阿里云 OSS、CDN 和证书管理服务中的证书。

## 功能特性

- 自动检测阿里云 OSS 自定义域名证书过期情况
- 基于 Let's Encrypt 自动申请/续期 SSL 证书
- 自动上传证书到阿里云证书管理服务 (CAS)
- 自动更新 OSS 自定义域名绑定的证书
- 自动更新 CDN 加速域名的证书
- 支持 Webhook 通知（企业微信、钉钉、飞书等）
- 支持多架构 Docker 镜像部署

## 快速开始

### 前置要求

- Go 1.23.0+ (如需从源码构建)
- 阿里云 Access Key ID 和 Access Key Secret
- 具有访问 OSS、CDN、CAS 服务权限的 RAM 账号

### 1. 下载安装

**方式一：使用预编译二进制**

从 [GitHub Releases](https://github.com/nekoimi/oss-auto-cert/releases) 下载对应平台的二进制文件。

**方式二：Docker 部署（推荐）**

```bash
docker pull ghcr.io/nekoimi/oss-auto-cert:latest
```

**方式三：源码构建**

```bash
git clone https://github.com/nekoimi/oss-auto-cert.git
cd oss-auto-cert
go build -o oss-auto-cert main.go
```

### 2. 配置环境变量

```bash
export OSS_ACCESS_KEY_ID="your-access-key-id"
export OSS_ACCESS_KEY_SECRET="your-access-key-secret"
```

### 3. 创建配置文件

创建 `config.yaml`：

```yaml
# 通知 Webhook（可选）
webhook: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxx

# 证书配置
acme:
  email: your-email@example.com
  data-dir: /var/lib/oss-auto-cert
  expired-early: 30

# OSS Bucket 列表
buckets:
  - name: my-bucket-1
    endpoint: oss-cn-hangzhou.aliyuncs.com
  - name: my-bucket-2
    endpoint: oss-cn-beijing.aliyuncs.com
```

### 4. 运行

**二进制运行：**

```bash
./oss-auto-cert -config=./config.yaml -log-level=info
```

**Docker 运行：**

```bash
docker run -d --rm \
  -v $PWD/config.yaml:/etc/oss-auto-cert/config.yaml \
  -e OSS_ACCESS_KEY_ID=$OSS_ACCESS_KEY_ID \
  -e OSS_ACCESS_KEY_SECRET=$OSS_ACCESS_KEY_SECRET \
  ghcr.io/nekoimi/oss-auto-cert:latest
```

## 配置详解

### 配置文件

配置文件支持 YAML 格式，完整配置示例：

```yaml
# 消息通知 Webhook 地址
webhook: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxx-xxxxx-xxxxx

# Webhook 请求体模板（可选）
# 默认使用企业微信格式，支持 Go 模板语法
webhook-tpl: |
  {
    "msgtype": "text",
    "text": {
      "content": "{{ .Message }}"
    }
  }

# ACME/Let's Encrypt 配置
acme:
  # 申请证书邮箱（必填）
  email: admin@example.com
  # 证书文件保存目录（绝对路径）
  # 默认: /var/lib/oss-auto-cert
  data-dir: /data/certs
  # 证书提前续期天数（默认: 15）
  # 推荐设置为 30 天
  expired-early: 30

# OSS Bucket 配置列表
buckets:
  - name: bucket-name-1
    endpoint: oss-cn-hangzhou.aliyuncs.com
  - name: bucket-name-2
    endpoint: oss-cn-beijing.aliyuncs.com
  - name: bucket-name-3
    endpoint: oss-cn-shenzhen.aliyuncs.com

# 七牛 Fusion CDN（可选）
qiniu:
  enabled: true
  domains:
    - domain: cdn.example.com
```

### 环境变量配置

环境变量优先级高于配置文件：

| 变量名 | 说明 | 必需 |
|--------|------|------|
| `OSS_ACCESS_KEY_ID` | 阿里云 Access Key ID | ✅ |
| `OSS_ACCESS_KEY_SECRET` | 阿里云 Access Key Secret | ✅ |
| `ACME_EMAIL` | 证书申请邮箱 | ❌ |
| `ACME_DATA_DIR` | 证书存储目录 | ❌ |
| `ACME_EXPIRED_EARLY` | 提前续期天数 | ❌ |
| `DEBUG` | 调试模式（true/false） | ❌ |
| `QINIU_ACCESS_KEY` | 七牛 AccessKey（启用七牛时必需） | ❌ |
| `QINIU_SECRET_KEY` | 七牛 SecretKey（启用七牛时必需） | ❌ |

### 命令行参数

```bash
./oss-auto-cert [参数]
```

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-config` | 配置文件路径 | `/etc/oss-auto-cert/config.yaml` |
| `-log-level` | 日志级别 (debug/info/warn/error) | `info` |

## 部署方式

### Docker 部署（推荐）

**基础运行：**

```bash
docker run -d --rm \
  --name oss-auto-cert \
  -v $PWD/config.yaml:/etc/oss-auto-cert/config.yaml \
  -e OSS_ACCESS_KEY_ID=xxx \
  -e OSS_ACCESS_KEY_SECRET=xxx \
  ghcr.io/nekoimi/oss-auto-cert:latest
```

**持久化证书：**

```bash
docker run -d --rm \
  --name oss-auto-cert \
  -v $PWD/config.yaml:/etc/oss-auto-cert/config.yaml \
  -v $PWD/certs:/var/lib/oss-auto-cert \
  -e OSS_ACCESS_KEY_ID=xxx \
  -e OSS_ACCESS_KEY_SECRET=xxx \
  ghcr.io/nekoimi/oss-auto-cert:latest
```

**使用 Docker Compose：**

创建 `docker-compose.yaml`：

```yaml
version: "3.8"
services:
  oss-auto-cert:
    image: ghcr.io/nekoimi/oss-auto-cert:latest
    container_name: oss-auto-cert
    hostname: oss-auto-cert
    command:
      - -log-level=info
    volumes:
      - ./config.yaml:/etc/oss-auto-cert/config.yaml
      - ./certs:/data/certs
    restart: unless-stopped
    environment:
      OSS_ACCESS_KEY_ID: ${OSS_ACCESS_KEY_ID}
      OSS_ACCESS_KEY_SECRET: ${OSS_ACCESS_KEY_SECRET}
      ACME_EMAIL: admin@example.com
      ACME_DATA_DIR: /data/certs
      ACME_EXPIRED_EARLY: 30
```

运行：

```bash
docker-compose up -d
```

### Systemd 部署

1. 下载二进制文件到 `/usr/bin/oss-auto-cert`

2. 创建配置文件 `/etc/oss-auto-cert/config.yaml`

3. 创建 systemd 服务文件 `/usr/lib/systemd/system/oss-auto-cert.service`：

```ini
[Unit]
Description=阿里云 OSS 证书自动化工具
Documentation=https://github.com/nekoimi/oss-auto-cert
After=network.target local-fs.target

[Service]
Type=simple
Environment="OSS_ACCESS_KEY_ID=your-key-id"
Environment="OSS_ACCESS_KEY_SECRET=your-key-secret"
ExecStart=/usr/bin/oss-auto-cert -config=/etc/oss-auto-cert/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

4. 启动服务：

```bash
systemctl daemon-reload
systemctl enable oss-auto-cert
systemctl start oss-auto-cert
```

5. 查看日志：

```bash
journalctl -u oss-auto-cert -f
```

## Webhook 通知配置

### 企业微信

```yaml
webhook: https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxx
webhook-tpl: |
  {
    "msgtype": "text",
    "text": {
      "content": "{{ .Message }}"
    }
  }
```

### 钉钉

```yaml
webhook: https://oapi.dingtalk.com/robot/send?access_token=xxxxx
webhook-tpl: |
  {
    "msgtype": "text",
    "text": {
      "content": "{{ .Message }}"
    }
  }
```

### 飞书

```yaml
webhook: https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx
webhook-tpl: |
  {
    "msg_type": "text",
    "content": {
      "text": "{{ .Message }}"
    }
  }
```

## 工作原理

1. **定时检测**：每 6 小时检测一次所有配置的 Bucket
2. **证书检查**：查询阿里云 CAS 获取证书过期时间
3. **过期判断**：根据 `expired-early` 设置判断是否需要续期
4. **证书申请**：使用 ACME/Let's Encrypt 申请新证书
5. **上传证书**：将新证书上传到阿里云证书管理服务
6. **更新服务**：自动更新 OSS 和 CDN 的证书绑定
7. **发送通知**：通过 Webhook 发送处理结果通知

## 权限要求

阿里云 RAM 账号需要以下权限：

- `AliyunOSSFullAccess` - OSS 对象存储管理权限
- `AliyunCDNFullAccess` - CDN 管理权限
- `AliyunCASFullAccess` - 证书管理服务权限

## 调试模式

设置环境变量开启调试模式：

```bash
export DEBUG=true
```

调试模式下：
- 证书过期检测会直接返回过期状态
- 使用 Let's Encrypt 测试环境 (staging)
- 适合测试配置和部署是否正确

## 常见问题

### Q: 证书续期频率是多少？

A: 工具每 6 小时检测一次，只在证书即将过期（默认提前 15 天）时才会申请新证书。

### Q: 如何查看日志？

A: 
- Docker: `docker logs oss-auto-cert`
- Systemd: `journalctl -u oss-auto-cert -f`
- 二进制: 直接查看终端输出或重定向到日志文件

### Q: 支持哪些证书类型？

A: 目前支持 Let's Encrypt 颁发的标准 SSL 证书，通配符证书支持取决于 DNS 提供商。

### Q: 证书存储在哪里？

A: 默认存储在 `/var/lib/oss-auto-cert`，可通过 `acme.data-dir` 或 `ACME_DATA_DIR` 环境变量修改。

### Q: 如何手动触发证书更新？

A: 临时设置 `DEBUG=true` 环境变量并重启服务，或使用 `--log-level=debug` 查看详细日志。

## 许可证

[LICENSE](LICENSE)

## 相关链接

- [GitHub 仓库](https://github.com/nekoimi/oss-auto-cert)
- [Let's Encrypt](https://letsencrypt.org/)
- [阿里云 OpenAPI](https://api.aliyun.com)
- [go-acme/lego](https://github.com/go-acme/lego)
