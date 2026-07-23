# Mulei Message Service

English · [简体中文](README.zh-CN.md)

A unified, high-availability message delivery service for SMS, email, WeCom, and DingTalk. Business applications integrate with one API while the platform handles templates, asynchronous queues, provider selection, failover, receipts, and delivery analytics.

**Technology stack:** Go 1.25+ · Redis Streams · MySQL/PostgreSQL · Vue · Ant Design Vue

![Mulei Message Service send-readiness dashboard](https://static.1ms.run/mdoc/uploads/2026/07/23/b813c058-4008-45a0-9561-9281272fcc28.png)

## Why Mulei Message Service?

Direct provider integrations are easy to start but become expensive to maintain when several applications, message types, or providers are involved. Mulei Message Service puts credentials, templates, routing, reliability policies, and delivery records behind a consistent application boundary.

## Highlights

- **Multiple channels:** SMS, email, WeCom, and DingTalk.
- **Flexible delivery:** single, batch, and scheduled messages.
- **Unified configuration:** system templates, provider accounts, provider templates, signatures, and sending channels.
- **High availability:** priority groups, smooth weighted round-robin, provider failover, circuit state, automatic disabling, and retry policies.
- **Asynchronous processing:** durable tasks, Redis Streams, worker pools, scheduled dispatch, and dead-letter handling.
- **Security and governance:** HMAC-SHA256 request signing, timestamps, nonces, replay protection, quotas, rate limits, and optional OIDC administrator login.
- **Traceability:** task history, provider requests and responses, delivery receipts, Webhook logs, upstream SMS, and statistics.

## Delivery Flow

```text
Business application
  → HMAC authentication, quota, and readiness checks
  → Template rendering and durable task creation
  → Redis Stream or scheduled queue
  → Channel selection and provider dispatch
  → Failure policy, receipt, Webhook, and delivery logs
```

## Local Demo

The local demo provides a complete SQLite database with deterministic fake data. It is intended for exploring the admin console and documentation screenshots without contacting real SMS, email, or bot services.

Prerequisites:

- Go 1.25 or later
- A local Redis instance

```bash
make demo-reset
make demo-run
```

After the service starts:

| Resource | Address or credential |
|---|---|
| Admin console | `http://localhost:8080/admin/` |
| Health check | `http://localhost:8080/health` |
| Username | `demo-admin` |
| Password | `demo-pass-2026` |

> The SQLite database, fake credentials, and fixed administrator account are for local use only. Never expose this demo environment to a network or use it in production.

## Production

Production installations use MySQL or PostgreSQL together with a dedicated Redis deployment. Configure independent database credentials, application encryption keys, JWT secrets, provider credentials, and OIDC settings where applicable.

Installation, API integration, deployment, security, and operations guidance is maintained in the [full documentation](https://mdoc.cc/mliev/message-push).

## Development

| Command | Purpose |
|---|---|
| `make dev` | Run the service in development mode |
| `make build` | Build `bin/push-service` |
| `make test` | Run all Go tests with coverage output |

## Documentation

Open the [Mulei Message Service documentation](https://mdoc.cc/mliev/message-push) for the installation guide, admin console manual, API authentication and examples, deployment instructions, troubleshooting, and architecture.

## License

Copyright © 2025 Hefei Muleiwu (Mliev) Information Technology Co., Ltd. See [LICENSE](LICENSE) for permitted use and restrictions.
