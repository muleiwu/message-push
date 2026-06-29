# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Mulei Message Service (mliev-push) is a high-performance, multi-channel message delivery service built in Go 1.25+. It supports SMS (Aliyun, Tencent Cloud, Zrwinfo), Email (SMTP), WeChat Work, and DingTalk. The service uses an async queue architecture with Redis Streams, a rule engine for intelligent failure handling, and provider auto-switching for high availability.

## Common Development Commands

### Database Migrations

Migrations run **automatically at server bootstrap** (the `migration` package runs goose `UpByOne` until current). Normally you don't run them manually — just `make dev`.

Migrations are **dialect-specific SQL files** embedded via `embed.FS`, organized by database into `migrations/mysql/`, `migrations/pgsql/`, and `migrations/sqlite/`. At runtime the runner (`migration/migration.go`) picks the subfolder matching `database.driver`. Each dialect folder holds the same set of goose-versioned files (identical `YYYYMMDDNNNNNN_*.sql` prefixes); `0001` builds the base schema and later files replay incremental changes. To add a migration, create the **same-named `.sql` file in all three folders** with `-- +goose Up` / `-- +goose Down` sections, keeping index names table-prefixed (SQLite requires globally-unique index names).

Note: the `make migrate-*` targets in the Makefile reference `cmd/migrate/main.go`, which does not exist in the current tree, so they fail. Migrations are applied via the bootstrap path and the install controller instead.

### Development
```bash
make dev             # Run in development mode (go run main.go)
make build           # Build binary to bin/push-service
make build-prod      # Optimized production build
```

### Testing & Code Quality
```bash
make test            # Run tests with coverage
make test-coverage   # Generate HTML coverage report
make fmt             # Format code
make lint            # Run golangci-lint
make deps            # Tidy and download dependencies
go test ./app/service/... -run TestFunctionName -v  # Run a single test
```

### Docker
```bash
make docker-build    # Build Docker image
make docker-up       # Start containers with docker-compose
make docker-down     # Stop containers
```

## High-Level Architecture

### Service Bootstrap Flow

The application uses a custom assembly-based dependency injection pattern:

1. **Entry Point**: `main.go` → `cmd.Start()` → `initializeServices()`
2. **Assembly Phase** (`config/assembly.go`): Initializes infrastructure in order via `interfaces.AssemblyInterface`
   - `envAssembly.Env` → Environment variables
   - `configAssembly.Config` → Configuration loader (Viper)
   - `loggerAssembly.Logger` → Zap logger
   - `databaseAssembly.Database` → GORM database
   - `redisAssembly.Redis` → Redis client
   - `cacheAssembly.Cache` → Cache layer
3. **Server Phase** (`config/server.go`): Starts services in order via `interfaces.ServerInterface`
   - `migration.Migration` → Database migrations (goose)
   - `worker_service.WorkerService` → Redis Stream consumer pool
   - `scheduler_service.SchedulerService` → Scheduled task dispatcher
   - `HttpServer` → Gin HTTP server

All assemblies and servers implement interfaces defined in `internal/interfaces/`. The bootstrap loop in `cmd/run.go` supports graceful reload via SIGHUP signal or API-triggered restart channel (`internal/pkg/reload`).

### Package Structure

- `app/` — Application layer (business logic, HTTP handlers, message processing)
  - `controller/` / `controller/admin/` — HTTP handlers (public API + admin API)
  - `service/` — Business logic services
  - `dao/` — Data access layer (GORM queries)
  - `model/` — Database models (GORM)
  - `dto/` — Data transfer objects (request/response structs)
  - `helper/` — Stateless utility helpers (crypto, JWT, signature, quota, templates, receiver validation). **Distinct from `internal/helper/`** — this is a collection of pure functions, not the DI singleton.
  - `constants/` — Shared enums/constants (message types, task status, error codes)
  - `middleware/` — HTTP middlewares (auth, rate limit, quota)
  - `sender/` — Provider-specific message senders
  - `selector/` — Channel selection algorithm
  - `worker/` — Worker pool and message handler
  - `queue/` — Redis Stream producer/consumer
  - `circuit/` — Circuit breaker
  - `registry/` — Provider registry
  - `scheduler/` — Scheduled task processing
- `internal/` — Infrastructure layer (framework-level code)
  - `helper/` — Global singleton providing Logger, Config, Database, Redis, Cache
  - `interfaces/` — Core interfaces (`HelperInterface`, `AssemblyInterface`, `ServerInterface`)
  - `pkg/` — Infrastructure packages (database, redis, cache, http_server, config, env, logger, reload)
  - `service/` — Internal services (migration, worker_service, scheduler_service)
- `config/` — Assembly and server wiring
  - `autoload/` — Configuration initializers (Go code, not YAML). Each file returns config maps loaded by Viper. Includes `router.go` (all route definitions), `middleware.go`, `database.go`, `redis.go`, etc.
- `migrations/` — Goose SQL migration files split by dialect into `mysql/`, `pgsql/`, `sqlite/` (named `YYYYMMDDNNNNNN_description.sql`); embedded via `migrations/embed.go`

### Helper Pattern

Access shared dependencies via `helper.GetHelper()` (singleton in `internal/helper/`):
- `GetLogger()` — gsr.Logger (Zap wrapper)
- `GetConfig()` — Viper configuration
- `GetDatabase()` — GORM DB instance
- `GetRedis()` — Redis client
- `GetCache()` — gsr.Cacher (cache interface)

### Message Processing Flow

```
HTTP Request (/api/v1/messages)
    → HMAC Authentication (auth_middleware)
    → Rate Limit / Quota Check (rate_limit_middleware, quota_middleware)
    → message_service.CreateTask() — Validates and creates PushTask
    → queue.Producer.Push() — Pushes to Redis Stream or scheduled sorted set
    → worker.WorkerPool (10 workers) — Consumes from Redis Stream
    → message_handler.Handle() — Processes message
    → selector.ChannelSelector.Select() — Smooth weighted round-robin with failover
    → sender.Factory.GetSender() — Returns provider-specific sender
    → sender.Send() — Calls external provider API
    → rule_engine_service.Evaluate() — Handles failures (retry/switch_provider/fail/alert)
    → callback_service.HandleCallback() — Processes provider status callbacks
```

### Key Interfaces

**Sender** (`app/sender/sender.go`):
- `Sender` — Core: `Send(ctx, req) -> *SendResponse` + `GetProviderCode() -> string`
- `BatchSender` — Optional: `BatchSend()` for bulk operations
- `CallbackHandler` — Optional: `HandleCallback()` for provider status callbacks
- `StatusQuerier` / `StatusPuller` — Optional: for status queries/pulls

**Factory** (`app/sender/factory.go`): Registry-based. `GetSender(providerCode)` returns the appropriate sender. Also provides `GetBatchSender()`, `GetCallbackHandler()` with capability checking.

**Rule Engine** (`app/service/rule_engine_service.go`):
- Scenes: `send_failure`, `callback_failure`
- Actions: `retry`, `switch_provider`, `fail`, `alert`
- Matches by: provider_code, message_type, error_code, error_keyword
- Caches rules in memory; call `RefreshCache()` after admin updates

### Channel Selector Algorithm

The `ChannelSelector` (`app/selector/channel_selector.go`) uses:
1. **Priority Groups** — Lower number = higher priority
2. **Smooth Weighted Round-Robin** — Within same priority group
3. **Provider Exclusion** — For rule engine switching
4. **5-Minute Provider Memory** — Same `(appID, channelID, receiver)` tuple avoids same provider within 5 min
5. **Circuit Breaker** — `status` (admin manual) and `is_active` (auto-disable on failures)

Weight state persists in Redis (24h TTL). Binding cache has 30s TTL. Call `InvalidateCacheForBinding()` after admin channel changes.

### HTTP API Structure

- `/api/v1/*` — Public API (HMAC-SHA256 signed via `X-App-Key`, `X-Timestamp`, `X-Signature` headers)
- `/api/admin/*` — Admin API (JWT authenticated)
- `/api/callback/:id` — Provider status callbacks (signature verified per provider)
- `/api/install/*` — First-time setup (blocked after installation)
- `/health` — Health check

All routes defined in `config/autoload/router.go`.

## Adding a New Message Provider

1. Create sender in `app/sender/{provider}_sender.go`: implement `Sender` interface, optionally `BatchSender`, `CallbackHandler`, `StatusQuerier`
2. Register in `app/sender/factory.go`: add to `NewFactory()` registration
3. Add provider code constant in `app/model/provider_template.go` if needed

## Configuration

YAML config files in `config/autoload/` are Go initializers that return Viper config maps. The actual YAML config is at `config.yaml` (copy from `config.yaml.example`). Environment variables override config values (e.g., `DATABASE_HOST`).

Key config areas: app, database, redis, logger, jwt, middleware (rate limits, quotas).

## Important Notes

- Graceful reload via `SIGHUP` signal or API endpoint (triggers full re-assembly)
- Database migrations use goose with dialect-specific SQL files in `migrations/{mysql,pgsql,sqlite}/` (embedded via `embed.FS`; runner selects the folder by `database.driver`)
- Rule engine caches rules in memory; admin changes require `RefreshCache()` or the admin API endpoint `/api/admin/failure-rules/refresh-cache`
- Channel selector caches bindings (30s TTL); call `InvalidateCacheForBinding()` after changes
- Worker pool uses Redis Streams consumer groups; failed messages go to dead letter queue
- Admin UI is a Vue.js app in `admin-webui/` (git submodule, Vben Admin + Ant Design Vue)