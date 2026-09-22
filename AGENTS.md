# AGENTS.md

This file provides guidance to Codex when working with code in this repository.

## Project Overview

Mulei Message Service (mliev-push) is a high-performance, multi-channel message delivery service built in Go 1.25+. It supports SMS (Aliyun, Tencent Cloud, Zrwinfo, Netease), Email (SMTP), WeChat Work (app + robot), DingTalk (app + robot), and QQ (OneBot 11). The service uses an async queue architecture with Redis Streams, a rule engine for intelligent failure handling, and provider auto-switching for high availability.

The service runs on the shared **go-web framework** (`cnb.cool/mliev/open/go-web`), which provides the DI container, the assembly/server bootstrap, Viper config, Zap logger, Redis, cache, and the Gin HTTP server. Domain logic is organized into DDD-style bounded-context modules under `modules/`; `app/` remains as the shared kernel (models/DAOs/DTOs) and HTTP application layer.

## Common Development Commands

### Database Migrations

Migrations run **automatically at server bootstrap** (the `migration` package runs goose `UpByOne` until current). Normally you don't run them manually — just `make dev`.

Migrations are **dialect-specific SQL files** embedded via `embed.FS`, organized by database into `migrations/mysql/`, `migrations/pgsql/`, and `migrations/sqlite/`. At runtime the runner (`migration/migration.go`) picks the subfolder matching `database.driver`. Each dialect folder holds the same set of goose-versioned files (identical `YYYYMMDDNNNNNN_*.sql` prefixes); `0001` builds the base schema and later files replay incremental changes. To add a migration, create the **same-named `.sql` file in all three folders** with `-- +goose Up` / `-- +goose Down` sections, keeping index names table-prefixed (SQLite requires globally-unique index names).

### Development
```bash
make dev             # Run in development mode (go run main.go start)
make build           # Build binary to bin/push-service
make build-prod      # Optimized linux production build
```

### Local Demo (SQLite + Redis DB 15)
```bash
make demo-reset      # Rebuild .local/message-push-demo.sqlite with deterministic fake data (cmd/demo)
make demo-run        # Start the service against the demo DB (login demo-admin / demo-pass-2026)
```

### Testing & Code Quality
```bash
make test                        # Run tests with coverage
make test-timezone-integration   # PostgreSQL/MySQL UTC migration round-trip tests (docker compose)
make test-coverage               # Generate HTML coverage report
make fmt                         # Format code
make lint                        # Run golangci-lint
make deps                        # Tidy and download dependencies
go test ./modules/delivery/... -run TestFunctionName -v  # Run a single test
```

Tests use the stdlib `testing` package only (no testify). Service/DAO tests typically seed an in-memory SQLite DB (`glebarez/sqlite`) with raw SQL `INSERT`s instead of mocks.

### Docker
```bash
make docker-build    # Build Docker image
make docker-up       # Start containers with docker-compose
make docker-down     # Stop containers
```

## High-Level Architecture

### Service Bootstrap Flow

`main.go` embeds `templates/` and `static/`, forces `time.Local = time.UTC`, and wraps go-web startup (`cmd.Start(cmd.WithApp(config.App{}))`) in `gomander.Run` (process supervisor from github.com/muleiwu/gomander). `config.App` implements the framework's AppProvider with two chains:

1. **Assembly Phase** (`config/app.go` → `Assemblies()`) — DI container wiring via `interfaces.AssemblyInterface`:
   - Framework infrastructure: env → config (from `config.Config.Get()` → `config/autoload/*`) → logger → database (`internal/database`) → redis → cache
   - Domain modules (`config/app_push.go` → `pushAssemblies()`): quota, sender, channel (+ catalog), ruleengine, delivery.Producer, template (renderer + service), messaging, callback, identity (application service + user service + OIDC service)
2. **Server Phase** (`config/app.go` → `Servers()`):
   - identity JWTConfigServer → `migration.Server` (goose) → identity BootstrapAdminServer (unattended initial-admin bootstrap) → delivery WorkerServer (Redis Stream consumer pool, **10 workers**; skipped when `app.installed` is false) → delivery SchedulerServer → callback WebhookServer → go-web HttpServer

### DDD Module Pattern

Each bounded context lives in `modules/{ctx}/` with this layout (use `modules/sender` or `modules/channel` as the template):

- `domain/` — ports (interfaces) + value objects; no SDK/DB imports. Referencing shared-kernel `app/model` types is OK.
- `infrastructure/` — concrete implementations of the ports (moved from the old `app/*` code, `package infrastructure`); assert compliance with `var _ domain.X = (*Impl)(nil)`.
- `assembly/` — go-web `AssemblyInterface` (`Type()` / `DependsOn()` / `Assembly()`), registers the port into the DI container by interface type.
- `{ctx}.go` — facade (package = ctx name): `type X = domain.X` aliases (keeps consumer churn near zero) + `GetX()` accessor via the container.

Register new assemblies in `config/app_push.go` `pushAssemblies()` and servers in `pushServers()`. Update consumers to resolve ports via the facade `GetX()`.

**GOTCHA — DependsOn:** go-web instantiates assemblies eagerly in topological order of `DependsOn()`, and `container.MustGet` does NOT lazily build during that phase (panics `provider X not found`). If an `Assembly()`/constructor touches `helper.GetLogger()/GetCache()/GetDatabase()` (directly or via a DAO), you MUST declare `DependsOn()` with the provider types actually used (`gsr.Logger`, `gsr.Cacher`, `*gorm.DB`).

Current modules: `sender`, `channel`, `ruleengine`, `delivery` (queue/worker/scheduler), `messaging`, `template`, `callback`, `identity`, `quota`.

### Package Structure

- `modules/` — DDD bounded contexts (see pattern above)
  - `sender/` — provider senders + factory (`infrastructure/factory.go`); Sender/Resolver/ConfigField ports in `domain/`
  - `channel/` — channel selector (`infrastructure/selector.go`) + public channel catalog (`infrastructure/catalog.go`)
  - `ruleengine/` — failure-rule evaluation (`infrastructure/engine.go`)
  - `delivery/` — queue producer/consumer (`infrastructure/queue/`, stream `push:stream` + dead letter `push:stream:dead_letter`), worker pool + message handler (`infrastructure/worker/`), scheduled-task scanner (`infrastructure/scheduler/`), `server/` (WorkerServer/SchedulerServer)
  - `messaging/` — application service: `Send` / `BatchSend` / `QueryTask`
  - `template/` — template renderer + template management service
  - `callback/` — provider status callback handling + WebhookServer
  - `identity/` — auth application service, user service, OIDC service, install/bootstrap admin services
  - `quota/` — quota checking service
- `app/` — shared kernel + HTTP application layer
  - `controller/` / `controller/admin/` — HTTP handlers (public + install + callback + admin)
  - `service/` — cross-context application services (admin CRUD/reporting: channel/log/provider account/provider resource/provider signature/statistics/task; plus `rule_action_executor`, `sms_event_service`, `sms_polling_service`, `task_terminal_service`, `admin_onboarding_service`)
  - `dao/` / `model/` / `dto/` / `constants/` — GORM models, queries, transfer objects, enums
  - `helper/` — stateless pure functions (crypto, JWT, signature, templates, receiver validation)
  - `middleware/` — auth, admin JWT, rate limit, quota, CORS, install check, body limit
  - `readiness/` — send-readiness checks (channel readiness, OneBot signature)
- `internal/` — `database/` (GORM bootstrap) and `timeutil/` (Asia/Shanghai business-calendar helpers)
- `config/` — `app.go` / `app_push.go` (assembly & server wiring), `config.go` + `autoload/` (Go initializers returning Viper config maps, incl. `router.go` with **all route definitions**)
- `migration/` — goose bootstrap runner + migration contract tests
- `migrations/` — dialect-specific SQL files (embedded)
- `cmd/demo/` — local demo DB seeder/reset tool
- `docs/` — `docs/channels/` (per-provider integration docs + capability matrix, one dir per sender), `docs/archify/` (generated architecture diagrams), `docs/timezone-audit.md`, provider-native-templates / provider-resource-management guides
- `admin-webui/` — git submodule (Vben Admin + Ant Design Vue)
- `templates/`, `static/` — embedded email templates and web static assets (landing page, admin UI, install pages)

### Helper Pattern

Access shared dependencies via the go-web helper singleton (`cnb.cool/mliev/open/go-web/pkg/helper`):
- `GetLogger()` — gsr.Logger (Zap wrapper)
- `GetConfig()` — Viper configuration
- `GetDatabase()` — GORM DB instance
- `GetRedis()` — Redis client
- `GetCache()` — gsr.Cacher (cache interface)

### Message Processing Flow

```
HTTP Request (/api/v1/messages)
    → HMAC Authentication (auth_middleware, X-App-Key/X-Timestamp/X-Signature)
    → Rate Limit / Quota Check (rate_limit_middleware, quota_middleware) + readiness checks
    → messaging.Service.Send() (modules/messaging) — validates and creates PushTask
    → delivery.Producer.Push() — pushes to Redis Stream or scheduled sorted set
    → delivery WorkerPool (10 workers) — consumes from Redis Stream
    → worker.MessageHandler.Handle() — processes message
    → channel.Selector.Select() — priority groups + smooth weighted round-robin with failover
    → sender factory (via sender.Resolver) — returns provider-specific sender
    → Sender.Send() — calls external provider API
    → ruleengine.Engine.Evaluate() — handles failures (retry/switch_provider/fail/alert)
    → callback handling (modules/callback + app/controller/callback_controller.go)
```

### Key Interfaces (in each module's `domain/`)

**Sender** (`modules/sender/domain/sender.go`):
- `Sender` — Core: `Send(ctx, req) -> *SendResponse` + `GetProviderCode() -> string`
- `BatchSender` — Optional: `BatchSend()` for bulk operations
- `CallbackHandler` — Optional: `HandleCallback()` for provider status callbacks
- `StatusQuerier` / `StatusPuller` — Optional: for status queries/pulls

Factory (`modules/sender/infrastructure/factory.go`): registry-based. `GetSender(providerCode)` plus `GetBatchSender()` / `GetCallbackHandler()` / `GetStatusQuerier()` / `GetStatusPuller()` with capability checking.

**Selector** (`modules/channel/domain/selector.go`): `Select()` / `SelectWithExcludes()` / `InvalidateCacheForBinding()` / `ResetWeightsByChannelID()` / `ReportSuccess()`.

**RuleEngine** (`modules/ruleengine/domain/engine.go`):
- Scenes: `send_failure`, `callback_failure`
- Actions: `retry`, `switch_provider`, `fail`, `alert`
- Matches by: provider_code, message_type, error_code, error_keyword
- Caches rules in memory; call `RefreshCache()` after admin updates

**Producer** (`modules/delivery/domain/producer.go`): `Producer` + `IdempotentProducer`.

**Messaging Service** (`modules/messaging/domain/service.go`): `Send` / `BatchSend` / `QueryTask`.

### Channel Selector Algorithm

The `ChannelSelector` (`modules/channel/infrastructure/selector.go`) uses:
1. **Priority Groups** — Lower number = higher priority
2. **Smooth Weighted Round-Robin** — Within same priority group
3. **Provider Exclusion** — For rule engine switching (`SelectWithExcludes`)
4. **5-Minute Provider Memory** — Same `(appID, channelID, receiver)` tuple avoids same provider within 5 min
5. **Circuit Breaker** — `status` (admin manual) and `is_active` (auto-disable on failures)

Weight state persists in Redis (24h TTL fallback; admin operations clear it actively). Call `InvalidateCacheForBinding()` (and `ResetWeightsByChannelID()` where weights matter) after admin channel changes — done by `app/service/admin_channel_service.go` and `modules/template`.

### HTTP API Structure

- `/` — Landing page
- `/health`, `/health/simple` — Health checks
- `/api/install/*` — First-time setup (blocked after installation)
- `/api/callback/:id` — Provider status callbacks (signature verified per provider)
- `/api/v1/channels` — Public channel catalog (for business-app message forms)
- `/api/v1/messages*` — Public send API (HMAC-SHA256 signed via `X-App-Key`, `X-Timestamp`, `X-Signature` headers)
- `/api/admin/auth/*` — Admin login/logout/OIDC SSO
- `/api/admin/*` — Admin API (JWT authenticated)

All routes defined in `config/autoload/router.go`.

## Adding a New Message Provider

1. Create sender in `modules/sender/infrastructure/{provider}_sender.go`: implement `domain.Sender`; optionally `BatchSender`, `CallbackHandler`, `StatusQuerier`, `StatusPuller`
2. Register in `modules/sender/infrastructure/factory.go`: add to `NewFactory()`
3. Add provider code constant / config field definitions in `modules/sender/domain/` (`config_field.go`) and `app/model/` as needed
4. Document it in `docs/channels/{provider}/README.md` (see `docs/channels/README.md` for the capability matrix and existing provider codes like `aliyun_sms`, `dingtalk_robot`, `onebot`)

## Configuration

YAML config files in `config/autoload/` are Go initializers that return Viper config maps (`Base`, `Http`, `Database`, `Redis`, `Migration`, `Middleware`, `Jwt`, `Oidc`, `EmailAttachments`, `Router`, ...). The actual YAML config is at `config.yaml` (copy from `config.yaml.example`). Environment variables override config values (e.g., `DATABASE_HOST`, `APP_INSTALLED`).

## Important Notes

- **Timezone:** the process runs UTC (`time.Local = time.UTC` in `main.go`); persistence and framework defaults are UTC. Business calendar operations explicitly use Asia/Shanghai through `internal/timeutil` — do not use `time.Local` in business logic. See `docs/timezone-audit.md` and `make test-timezone-integration`.
- Migration, worker, and scheduler servers **skip startup when `app.installed` is false** (pre-install state)
- Unattended initial-admin bootstrap (k8s friendly) via `APP_INSTALLED` / `ADMIN_*` env vars (identity `BootstrapAdminServer`)
- Rule engine caches rules in memory; admin changes require `RefreshCache()` or the admin API endpoint `/api/admin/failure-rules/refresh-cache`
- Channel selector caches bindings; call `InvalidateCacheForBinding()` after changes (see `app/service/admin_channel_service.go`)
- Worker pool uses Redis Streams consumer groups (`push:stream`); failed messages go to the dead letter stream `push:stream:dead_letter`
- Admin UI is a Vue.js app in `admin-webui/` (git submodule, Vben Admin + Ant Design Vue)
- Demo assets are ensured by `scripts/ensure-demo-assets.sh`; other scripts in `scripts/` cover API signing (apifox), doc publishing, and manual verification
- `CLAUDE.md` mirrors this file for Claude Code — keep the two in sync when changing guidance
- CI is `.cnb.yml` (cnb.cool pipeline: multi-arch builds amd64/arm/loong64 + image push) — there is no GitHub Actions workflow
