# 木雷消息服务

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org)
[![Gin Framework](https://img.shields.io/badge/Gin-v1.10-00ADD8.svg)](https://github.com/gin-gonic/gin)
[![License](https://img.shields.io/badge/License-Proprietary-blue.svg)](LICENSE)

一个基于 Go 语言构建的高性能、高可用多通道消息推送服务。支持短信（阿里云、腾讯云、掌榕网）、邮件、企业微信、钉钉等多种通道，内置智能规则引擎实现发送失败自动重试与供应商切换，配合服务商回调处理确保消息可靠投递。具备异步队列、熔断器、限流、配额管理等企业级特性，提供完善的管理后台和 API。

[English](README.md)

## 特性

- **多通道支持** - 短信、邮件、企业微信、钉钉、Webhook
- **多短信服务商** - 阿里云、腾讯云、掌榕网，支持自动故障转移
- **异步处理** - 基于 Redis Stream 的消息队列，Worker Pool 架构
- **高可用** - 熔断器模式、平滑加权轮询负载均衡
- **规则引擎** - 灵活的失败处理规则，支持自定义动作
- **供应商自动切换** - 发送失败时自动切换到备用供应商
- **回调处理** - 处理服务商状态回调，支持 Webhook 通知
- **企业级** - 限流、配额管理、指数退避重试
- **灵活投递** - 单条、批量、定时消息发送
- **安全 API** - HMAC-SHA256 签名认证
- **管理后台** - 基于 Vue.js + Ant Design 的管理界面
- **模板引擎** - 支持变量替换的可复用消息模板

## 支持的服务商

| 通道 | 服务商 | 特性 |
|------|--------|------|
| 短信 | 阿里云短信 | 国内、国际 |
| 短信 | 腾讯云短信 | 国内、国际 |
| 短信 | 掌榕网 | 国内 |
| 邮件 | SMTP | 标准 SMTP 协议 |
| 即时通讯 | 企业微信 | 应用消息 |
| 即时通讯 | 钉钉 | 机器人消息 |

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP API 层                              │
│                  (认证 / 限流 / 配额检查)                         │
├─────────────────────────────────────────────────────────────────┤
│                        消息服务层                                │
│                  (创建任务 → 推送到队列)                          │
├─────────────────────────────────────────────────────────────────┤
│                     Redis Stream 队列                           │
├─────────────────────────────────────────────────────────────────┤
│                      Worker Pool (N)                            │
│                (消费 → 处理 → 发送 → 更新状态)                    │
├─────────────────────────────────────────────────────────────────┤
│                        通道选择器                                │
│               (平滑加权轮询 / 故障转移)                           │
├─────────────────────────────────────────────────────────────────┤
│                         规则引擎                                 │
│              (失败处理 / 重试 / 切换供应商)                       │
├─────────────────────────────────────────────────────────────────┤
│                       服务商发送器                               │
│        (阿里云 / 腾讯云 / 掌榕网 / SMTP / 企业微信)               │
├─────────────────────────────────────────────────────────────────┤
│                        回调处理层                                │
│               (状态回调 / Webhook 通知)                          │
├─────────────────────────────────────────────────────────────────┤
│                         规则引擎                                 │
│              (失败处理 / 重试 / 切换供应商)                       │
└─────────────────────────────────────────────────────────────────┘
```

## 快速开始

### 前置条件

- Go 1.21+
- MySQL 5.7+ 或 PostgreSQL
- Redis 5.0+

### 1. 克隆并配置

```bash
git clone https://cnb.cool/mliev/push/message-push
cd message-push

# 复制并编辑配置文件
cp config.yaml.example config.yaml
```

### 2. 配置数据库

编辑 `config.yaml`:

```yaml
database:
  host: localhost
  port: 3306
  username: root
  password: your_password
  database: push_service

redis:
  host: localhost
  port: 6379
```

### 3. 启动服务

```bash
# 开发模式
go run main.go

# 或者编译后运行
make build
./bin/push-service
```

服务将在 `http://localhost:8080` 启动

### 4. 验证安装

```bash
# 健康检查
curl http://localhost:8080/health

# 预期响应
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "UP",
    "services": {
      "database": {"status": "UP"},
      "redis": {"status": "UP"}
    }
  }
}
```

## API 使用

### 签名认证

所有 API 请求需要 HMAC-SHA256 签名：

```
签名 = HMAC-SHA256(app_secret, "app_key={app_key}&timestamp={timestamp}")
```

请求头：
- `X-App-Key`: 应用密钥
- `X-Timestamp`: Unix 时间戳
- `X-Signature`: 生成的签名

### 发送单条消息

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "sms",
    "receiver": "13800138000",
    "template_code": "verify_code",
    "template_params": {
      "code": "123456"
    }
  }'
```

### 批量发送消息

```bash
curl -X POST http://localhost:8080/api/v1/messages/batch \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "sms",
    "receivers": ["13800138000", "13800138001"],
    "template_code": "promotion",
    "template_params": {
      "activity": "双十一"
    }
  }'
```

### 查询任务状态

```bash
curl http://localhost:8080/api/v1/messages/{task_id} \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature"
```

完整 API 文档请参阅 [API 指南](docs/API_GUIDE.md)。

## 规则引擎

规则引擎提供智能的失败处理机制，支持自定义规则配置：

### 支持的场景

| 场景 | 说明 |
|------|------|
| `send_failure` | 消息发送失败时触发 |
| `callback_failure` | 服务商回调报告失败时触发 |

### 支持的动作

| 动作 | 说明 |
|------|------|
| `retry` | 重试，支持配置延迟和指数退避 |
| `switch_provider` | 切换到其他供应商重试 |
| `fail` | 直接标记任务失败 |
| `alert` | 通过 Webhook 发送告警通知 |

### 匹配条件

规则支持多种匹配条件：

- **供应商代码** - 匹配特定供应商（如 `aliyun`、`tencent`）
- **消息类型** - 匹配消息类型（如 `sms`、`email`）
- **错误码** - 匹配特定错误码（支持逗号分隔多个）
- **错误关键字** - 模糊匹配错误消息内容

### 规则配置示例

```json
{
  "name": "网络超时重试",
  "scene": "send_failure",
  "provider_code": "",
  "message_type": "sms",
  "error_keyword": "timeout,network",
  "action": "retry",
  "action_config": {
    "max_retry": 3,
    "delay_seconds": 5,
    "backoff_rate": 2
  },
  "priority": 100
}
```

## 配置说明

### 主要配置项

```yaml
# 服务器设置
server:
  mode: release          # debug, release, test
  addr: ":8080"

# 数据库
database:
  host: localhost
  port: 3306
  username: root
  password: password
  database: push_service
  max_open_conns: 100
  max_idle_conns: 10

# Redis（消息队列）
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 10

# JWT（管理后台认证）
jwt:
  secret: your-secret-key
  expire_hours: 24

# 限流配置
rate_limit:
  requests_per_minute: 100
  burst: 10
```

## 项目结构

```
message-push/
├── main.go                 # 程序入口
├── cmd/
│   ├── migrate/           # 数据库迁移工具
│   └── run.go             # 服务启动
├── config/
│   ├── config.go          # 配置加载器
│   ├── assembly.go        # 依赖注入
│   └── autoload/          # 自动加载的配置
├── app/
│   ├── controller/        # HTTP 处理器
│   │   └── admin/         # 管理后台 API
│   ├── service/           # 业务逻辑
│   ├── dao/               # 数据访问层
│   ├── model/             # 数据库模型
│   ├── dto/               # 数据传输对象
│   ├── middleware/        # HTTP 中间件
│   ├── sender/            # 消息发送器
│   ├── queue/             # Redis 队列 (生产者/消费者)
│   ├── worker/            # Worker Pool
│   ├── selector/          # 通道选择器
│   ├── circuit/           # 熔断器
│   └── helper/            # 工具函数
├── admin-webui/           # Vue.js 管理后台
├── docs/                  # 文档
└── deploy/                # 部署配置
```

## 部署

### Docker Compose

```yaml
# docker-compose.yml
services:
  mliev-push:
    container_name: mliev-push
    image: docker.cnb.cool/mliev/open/mliev-push:develop
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - "./config/:/app/config/"
```

```bash
docker compose up -d
```

### Kubernetes

```bash
kubectl apply -f deploy/kubernetes/
```

生产环境部署指南请参阅 [生产部署文档](docs/PRODUCTION_DEPLOYMENT.md)。

## 文档

- [快速启动指南](docs/QUICKSTART.md)
- [API 指南](docs/API_GUIDE.md)
- [API 集成文档](docs/API_INTEGRATION.md)
- [安装指南](docs/INSTALL_GUIDE.md)
- [生产部署](docs/PRODUCTION_DEPLOYMENT.md)
- [项目规范](docs/PROJECT_SPECIFICATION.md)

## SDKs

| 语言 | 仓库 | 说明 |
|------|------|------|
| Go | [push-go](https://github.com/mliev-sdk/push-go) | Go SDK，支持 context 和完整的错误处理 |
| PHP | [push-php](https://github.com/mliev-sdk/push-php) | PHP SDK，支持 PSR-18 HTTP 客户端 |

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.21+ |
| Web 框架 | Gin v1.10 |
| ORM | GORM v1.25 |
| 数据库 | MySQL / PostgreSQL |
| 缓存与队列 | Redis |
| 配置管理 | Viper |
| 日志 | Zap |
| 管理后台 | Vue.js + Ant Design Vue |

## 贡献

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m '添加新特性'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 社区

| QQ 群 | 微信 |
|:-----:|:----:|
| ![QQ](https://static.1ms.run/dwz/image/httpsn3.inklmKc.png) | ![微信](https://static.1ms.run/dwz/image/wechat_qr_code.png) |
| 群号: 1021660914 | 扫码加入 |

## 许可证

版权所有 © 2025 合肥木雷坞信息技术有限公司 保留所有权利

本软件基于自定义授权协议发布。可免费用于商业和非商业用途，但有使用限制。您必须保留版权标识和"Powered by"标识，禁止重新分发派生版本。

完整条款请参阅 [LICENSE](LICENSE) 文件。

