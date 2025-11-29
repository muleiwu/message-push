# Mulei Message Service

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org)
[![Gin Framework](https://img.shields.io/badge/Gin-v1.10-00ADD8.svg)](https://github.com/gin-gonic/gin)
[![License](https://img.shields.io/badge/License-Proprietary-blue.svg)](LICENSE)

A high-performance, highly available multi-channel message service built with Go. Supports SMS (Aliyun, Tencent Cloud, Zrwinfo), Email, WeChatWork, DingTalk and more. Features an intelligent rule engine for automatic retry and provider switching on failures, combined with provider callback processing to ensure reliable message delivery. Includes enterprise-grade features such as async queue processing, circuit breaker, rate limiting, quota management, and a comprehensive admin dashboard.

[中文文档](README.zh-CN.md)

## Features

- **Multi-Channel Support** - SMS, Email, WeChatWork, DingTalk, Webhook
- **Multiple SMS Providers** - Aliyun, Tencent Cloud, Zrwinfo with automatic failover
- **Async Processing** - Redis Stream based message queue with worker pool
- **High Availability** - Circuit breaker pattern, smooth weighted round-robin load balancing
- **Rule Engine** - Flexible failure handling rules with customizable actions
- **Provider Auto-Switching** - Automatically switch to alternative providers on failure
- **Callback Processing** - Handle provider status callbacks with webhook notifications
- **Enterprise Ready** - Rate limiting, quota management, retry with exponential backoff
- **Flexible Delivery** - Single, batch, and scheduled message sending
- **Secure API** - HMAC-SHA256 signature authentication
- **Admin Dashboard** - Vue.js + Ant Design based management UI
- **Template Engine** - Reusable message templates with variable substitution

## Supported Providers

| Channel | Provider | Features |
|---------|----------|----------|
| SMS | Aliyun SMS | Domestic & International |
| SMS | Tencent Cloud SMS | Domestic & International |
| SMS | Zrwinfo | Domestic |
| Email | SMTP | Standard SMTP protocol |
| IM | WeChatWork | Application messages |
| IM | DingTalk | Robot messages |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP API Layer                           │
│              (Authentication / Rate Limit / Quota)              │
├─────────────────────────────────────────────────────────────────┤
│                      Message Service                            │
│                (Create Task → Push to Queue)                    │
├─────────────────────────────────────────────────────────────────┤
│                      Redis Stream Queue                         │
├─────────────────────────────────────────────────────────────────┤
│                       Worker Pool (N)                           │
│              (Consume → Process → Send → Update)                │
├─────────────────────────────────────────────────────────────────┤
│                     Channel Selector                            │
│            (Smooth Weighted Round-Robin / Failover)             │
├─────────────────────────────────────────────────────────────────┤
│                        Rule Engine                              │
│         (Failure Handling / Retry / Switch Provider)            │
├─────────────────────────────────────────────────────────────────┤
│                    Provider Senders                             │
│        (Aliyun / Tencent / Zrwinfo / SMTP / WeChatWork)         │
├─────────────────────────────────────────────────────────────────┤
│                   Callback Processing                           │
│          (Status Callbacks / Webhook Notifications)             │
├─────────────────────────────────────────────────────────────────┤
│                        Rule Engine                              │
│         (Failure Handling / Retry / Switch Provider)            │
└─────────────────────────────────────────────────────────────────┘
```

![Architecture](https://static.1ms.run/image/message-push/mermaid-20251130%20005951.jpeg)

## Quick Start

### Prerequisites

- Go 1.21+
- MySQL 5.7+ or PostgreSQL
- Redis 5.0+

### 1. Clone and Configure

```bash
git clone https://cnb.cool/mliev/push/message-push
cd message-push

# Copy and edit configuration
cp config.yaml.example config.yaml
```

### 2. Configure Database

Edit `config.yaml`:

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

### 3. Start Service

```bash
# Development
go run main.go

# Or build and run
make build
./bin/push-service
```

The service will start on `http://localhost:8080`

### 4. Verify Installation

```bash
# Health check
curl http://localhost:8080/health

# Expected response
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

## API Usage

### Authentication

All API requests require HMAC-SHA256 signature:

```
Signature = HMAC-SHA256(app_secret, "app_key={app_key}&timestamp={timestamp}")
```

Headers:
- `X-App-Key`: Your application key
- `X-Timestamp`: Unix timestamp
- `X-Signature`: Generated signature

### Send Single Message

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

### Send Batch Messages

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
      "activity": "Black Friday"
    }
  }'
```

### Query Task Status

```bash
curl http://localhost:8080/api/v1/messages/{task_id} \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature"
```

For complete API documentation, see [API Guide](docs/API_GUIDE.md).

## Rule Engine

The Rule Engine provides intelligent failure handling with customizable rules:

### Supported Scenes

| Scene | Description |
|-------|-------------|
| `send_failure` | Triggered when message sending fails |
| `callback_failure` | Triggered when provider callback reports failure |

### Supported Actions

| Action | Description |
|--------|-------------|
| `retry` | Retry with configurable delay and exponential backoff |
| `switch_provider` | Switch to an alternative provider and retry |
| `fail` | Mark the task as failed immediately |
| `alert` | Send alert notification via webhook |

### Matching Conditions

Rules can be configured with multiple matching conditions:

- **Provider Code** - Match specific provider (e.g., `aliyun`, `tencent`)
- **Message Type** - Match message type (e.g., `sms`, `email`)
- **Error Code** - Match specific error codes (supports comma-separated list)
- **Error Keyword** - Fuzzy match on error message content

### Example Rule Configuration

```json
{
  "name": "Retry on network timeout",
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

## Configuration

### Key Configuration Options

```yaml
# Server settings
server:
  mode: release          # debug, release, test
  addr: ":8080"

# Database
database:
  host: localhost
  port: 3306
  username: root
  password: password
  database: push_service
  max_open_conns: 100
  max_idle_conns: 10

# Redis (message queue)
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
  pool_size: 10

# JWT (admin authentication)
jwt:
  secret: your-secret-key
  expire_hours: 24

# Rate limiting
rate_limit:
  requests_per_minute: 100
  burst: 10
```

## Project Structure

```
message-push/
├── main.go                 # Entry point
├── cmd/
│   ├── migrate/           # Database migration tool
│   └── run.go             # Server bootstrap
├── config/
│   ├── config.go          # Configuration loader
│   ├── assembly.go        # Dependency injection
│   └── autoload/          # Auto-loaded configs
├── app/
│   ├── controller/        # HTTP handlers
│   │   └── admin/         # Admin API handlers
│   ├── service/           # Business logic
│   ├── dao/               # Data access layer
│   ├── model/             # Database models
│   ├── dto/               # Data transfer objects
│   ├── middleware/        # HTTP middlewares
│   ├── sender/            # Message senders
│   ├── queue/             # Redis queue (producer/consumer)
│   ├── worker/            # Worker pool
│   ├── selector/          # Channel selector
│   ├── circuit/           # Circuit breaker
│   └── helper/            # Utilities
├── admin-webui/           # Vue.js admin dashboard
├── docs/                  # Documentation
└── deploy/                # Deployment configs
```

## Deployment

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

For production deployment guide, see [Production Deployment](docs/PRODUCTION_DEPLOYMENT.md).

## Documentation

- [Quick Start Guide](docs/QUICKSTART.md)
- [API Guide](docs/API_GUIDE.md)
- [API Integration](docs/API_INTEGRATION.md)
- [Installation Guide](docs/INSTALL_GUIDE.md)
- [Production Deployment](docs/PRODUCTION_DEPLOYMENT.md)
- [Project Specification](docs/PROJECT_SPECIFICATION.md)

## SDKs

| Language | Repository | Description |
|----------|------------|-------------|
| Go | [push-go](https://github.com/mliev-sdk/push-go) | Go SDK with context support and comprehensive error handling |
| PHP | [push-php](https://github.com/mliev-sdk/push-php) | PHP SDK with PSR-18 HTTP client support |

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.21+ |
| Web Framework | Gin v1.10 |
| ORM | GORM v1.25 |
| Database | MySQL / PostgreSQL |
| Cache & Queue | Redis |
| Configuration | Viper |
| Logging | Zap |
| Admin UI | Vue.js + Ant Design Vue |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Community

| QQ Group | WeChat |
|:--------:|:------:|
| ![QQ](https://static.1ms.run/dwz/image/httpsn3.inklmKc.png) | ![WeChat](https://static.1ms.run/dwz/image/wechat_qr_code.png) |
| Group: 1021660914 | Scan to join |

## License

Copyright © 2025 Hefei Muleiwu(Mliev) Information Technology Co., Ltd. All Rights Reserved.

This software is provided under a proprietary license. Free for commercial and non-commercial use with restrictions. You must retain copyright notices and "Powered by" marks. Redistribution of derivative works is prohibited.

See the [LICENSE](LICENSE) file for complete terms.
