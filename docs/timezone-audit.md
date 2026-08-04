# 数据库时间字段与时区语义审计

> 状态：本文保留的是 UTC 修复前的审计基线。`20260804000001_normalize_timepoints_utc.sql` 已完成整改：PostgreSQL 的 50 个时间点列改为 `TIMESTAMPTZ`，MySQL/SQLite 连接与值统一 UTC，2 个 `stat_date` 继续作为上海业务日期。发布步骤见 [UTC 时间迁移发布手册](timezone-rollout.md)。下文的“当前”均指整改前代码。

审计日期：2026-08-04
审计范围：当前工作树中的三套 Goose 迁移、`app/model` GORM 模型、仓库内时间写入/读取代码，以及 `go.mod` 锁定的数据库装配与驱动源码。本文描述的是**源码定义的目标 schema 与运行时语义**，不是对某个线上实例执行 `information_schema` 后得到的实库盘点。

## 结论

如果“记录时区”指数据库列能保留每条记录原始的时区/UTC offset，那么结论是：**当前 19 张表的 52 个日期/时间列都没有这种可靠能力。**

- PostgreSQL 的 50 个时间点字段全部是裸 `TIMESTAMP`，没有 `TIMESTAMPTZ` / `TIMESTAMP WITH TIME ZONE`；另外 2 个统计日字段是 `DATE`。最终列值只保留墙上时间，不能保留 Go `time.Time` 原始 location/offset（锁定的 pgx `Timestamp` 类型也明确把时区视为可丢弃信息）。
- MySQL 的 50 个时间点字段全部是 `TIMESTAMP`，2 个统计日字段是 `DATE`。`TIMESTAMP` 可受数据库 session `time_zone` 影响，但不会保留“原始输入是 Asia/Shanghai 还是 +08:00”这样的逐行信息。当前 MySQL 客户端只设置了 Go 侧 `loc=Local` / `ParseTime=true`，没有设置 MySQL session `time_zone`。
- SQLite 的 schema 使用 50 个 `DATETIME` 和 2 个 `DATE`，列声明本身不保证时区。不过这里有一个重要例外：锁定的 SQLite 驱动把显式传入的 Go `time.Time` 默认格式化为带 `±HH:MM` 的文本。因此 **SQLite 某些实际值会带数值 offset**，但这来自序列化文本，不是 schema 级的统一约束；同一列还可能由 `CURRENT_TIMESTAMP` 默认值或历史 SQL 写入，不能据此认定整列都有可靠时区。
- PostgreSQL runtime session 固定为 `Asia/Shanghai`，只是在裸 `TIMESTAMP` 的写入、默认值及日期分桶中提供统一的墙上时间语义；**session 时区不等于列记录了时区**。
- API 中不少时间被格式化为 RFC 3339 并带 offset；这个 offset 是数据库扫描成 Go `time.Time` 后的 location，不一定是最初写入者的原始时区。

审计计数为：**19 张表、52 个日期/时间列 = 50 个时间点列 + 2 个纯日期列**。三种方言的字段集合一致。

## 判定口径

本文区分三个容易混淆的概念：

1. **列是否携带时区**：是否使用可表达绝对时间语义的类型或另存 `timezone` / `utc_offset`。本项目没有这类列。
2. **连接/session 时区**：数据库或驱动用哪个时区解释不带 offset 的墙上时间。它会影响写入、读取或 `CURRENT_TIMESTAMP`，但不会保存每行原始时区。
3. **Go 值的 location**：`time.Time` 在内存中带 location/offset。经过不带时区的数据库类型编码后，原 location 可能丢失；读取时又可能附加一个默认 location。

`stat_date` 是业务日历日期，不是时间点；它使用 `DATE` 是合理的，时区只影响“先按哪个时区决定今天是哪一天”。`cost_time`、`response_time`、`timeout`、`timeout_seconds` 是时长，不是时间点，见文末单列说明。

## 完整字段/类型矩阵

下表按执行全部 `Up` 迁移后的当前 schema 汇总。所有 PostgreSQL `TIMESTAMP` 均为裸类型；所有模型时间点均为 `time.Time`、`*time.Time` 或包装 `time.Time` 的 `gorm.DeletedAt`。

| 表 | 日期/时间字段（完整） | Go 模型 | MySQL | PostgreSQL | SQLite | 主要源码证据 |
|---|---|---|---|---|---|---|
| `applications` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L20-L22](../app/model/application.go#L20-L22)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L6-L18) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L6-L18) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L6-L18) |
| `provider_accounts` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L20-L22](../app/model/provider_account.go#L20-L22)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L24-L35) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L24-L35) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L24-L35) |
| `provider_signatures` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L17-L19](../app/model/provider_signature.go#L17-L19)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L42-L51) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L42-L51) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L42-L51) |
| `channels` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L16-L18](../app/model/channel.go#L16-L18)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L57-L65) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L57-L65) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L57-L65) |
| `push_tasks` | `callback_time`, `scheduled_at`, `created_at`, `updated_at` | `*time.Time`, `*time.Time`, `time.Time`, `time.Time` | 4 × `TIMESTAMP` | 4 × `TIMESTAMP` | 4 × `DATETIME` | [模型 L24-L30](../app/model/push_task.go#L24-L30)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L72-L93) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L72-L93) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L72-L93) |
| `push_batch_tasks` | `created_at`, `updated_at` | 2 × `time.Time` | 2 × `TIMESTAMP` | 2 × `TIMESTAMP` | 2 × `DATETIME` | [模型 L17-L18](../app/model/push_batch_task.go#L17-L18)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L102-L112) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L102-L112) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L102-L112) |
| `push_logs` | `created_at` | `time.Time` | `TIMESTAMP` | `TIMESTAMP` | `DATETIME` | [模型 L19](../app/model/push_log.go#L19)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L118-L128) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L118-L128) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L118-L128) |
| `message_templates` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L19-L21](../app/model/message_template.go#L19-L21)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L136-L147) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L136-L147) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L136-L147) |
| `provider_templates` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L21-L23](../app/model/provider_template.go#L21-L23)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L153-L164) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L153-L164) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L153-L164) |
| `channel_template_bindings` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L39-L41](../app/model/channel_template_binding.go#L39-L41)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L169-L184) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L169-L184) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L169-L184) |
| `channel_signature_mappings` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L17-L19](../app/model/channel_signature_mapping.go#L17-L19)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L190-L199) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L190-L199) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L190-L199) |
| `channel_health_history` | `check_time`, `created_at` | 2 × `time.Time` | 2 × `TIMESTAMP` | 2 × `TIMESTAMP` | 2 × `DATETIME` | [模型 L11-L17](../app/model/channel_health_history.go#L11-L17)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L205-L214) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L205-L214) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L205-L214) |
| `app_quota_stats` | `stat_date`; `created_at`, `updated_at` | 3 × `time.Time` | `DATE`; 2 × `TIMESTAMP` | `DATE`; 2 × `TIMESTAMP` | `DATE`; 2 × `DATETIME` | [模型 L11-L16](../app/model/app_quota_stat.go#L11-L16)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L219-L227) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L219-L227) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L219-L227) |
| `provider_quota_stats` | `stat_date`; `created_at`, `updated_at` | 3 × `time.Time` | `DATE`; 2 × `TIMESTAMP` | `DATE`; 2 × `TIMESTAMP` | `DATE`; 2 × `DATETIME` | [模型 L11-L16](../app/model/provider_quota_stat.go#L11-L16)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L232-L240) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L232-L240) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L232-L240) |
| `admin_users` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L19-L21](../app/model/admin_user.go#L19-L21)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L245-L253) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L245-L253) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L245-L253) |
| `webhook_configs` | `created_at`, `updated_at` | 2 × `time.Time` | 2 × `TIMESTAMP` | 2 × `TIMESTAMP` | 2 × `DATETIME` | [模型 L18-L19](../app/model/webhook_config.go#L18-L19)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L259-L270) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L259-L270) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L259-L270) |
| `callback_logs` | `created_at` | `time.Time` | `TIMESTAMP` | `TIMESTAMP` | `DATETIME` | [模型 L21](../app/model/callback_log.go#L21)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L274-L284) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L274-L284) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L274-L284) |
| `webhook_logs` | `next_attempt_at`, `locked_until`, `created_at`, `updated_at` | `*time.Time`, `*time.Time`, `time.Time`, `time.Time` | 4 × `TIMESTAMP` | 4 × `TIMESTAMP` | 4 × `DATETIME` | [模型 L25-L29](../app/model/webhook_log.go#L25-L29)；初始 `created_at`：[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L292-L305) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L292-L305) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L292-L305)；新增三列：[MySQL L7-L10](../migrations/mysql/20260730000001_webhook_outbox.sql#L7-L10) / [PG L6-L9](../migrations/pgsql/20260730000001_webhook_outbox.sql#L6-L9) / [SQLite L6-L9](../migrations/sqlite/20260730000001_webhook_outbox.sql#L6-L9) |
| `failure_rules` | `created_at`, `updated_at`, `deleted_at` | `time.Time`, `time.Time`, `gorm.DeletedAt` | 3 × `TIMESTAMP` | 3 × `TIMESTAMP` | 3 × `DATETIME` | [模型 L38-L40](../app/model/failure_rule.go#L38-L40)；[MySQL](../migrations/mysql/20241201000001_initial_schema.sql#L312-L327) / [PG](../migrations/pgsql/20241201000001_initial_schema.sql#L312-L327) / [SQLite](../migrations/sqlite/20241201000001_initial_schema.sql#L312-L327) |

三套迁移会按 `database.driver` 选择并嵌入运行，见 [migrations/embed.go L5-L16](../migrations/embed.go#L5-L16) 与 [migration/migration.go L107-L128](../migration/migration.go#L107-L128)。本次也搜索了所有后续迁移的 `Up` 段；除 `webhook_logs` 上述三列外，没有新增其他日期/时间列。

## 方言与连接语义

### PostgreSQL

1. 迁移只声明 `TIMESTAMP`，没有 `TIMESTAMPTZ`。锁定的 pgx `Timestamp` 源码注释为“编码到 PostgreSQL 时忽略时区”，其类型编码会调用 `discardTimeZone`，保留年月日时分秒、丢弃原 offset：`github.com/jackc/pgx/v5@v5.9.2/pgtype/timestamp.go:27-30, 165-180, 194-218, 234-239`。当前连接启用了 simple protocol；显式 Go `time.Time` 参数也可能先按 `timestamptz` 文本发送、再由 PostgreSQL cast 到目标裸 `TIMESTAMP`，但最终列同样不可能保存原 zone/offset。精确 cast 后的墙上时间需连接实测，不能由 schema 单独保证。
2. 项目通过 [go.mod L6、L25-L26](../go.mod#L6) 锁定 `go-web v1.8.4`、PostgreSQL driver 与 GORM。`go-web@v1.8.4/pkg/server/database/config/config.go:45-59` 在 DSN 中固定 `TimeZone=Asia/Shanghai`；仓库自己的安装连接测试也同样固定它，[install_service.go L96-L103](../modules/identity/infrastructure/install_service.go#L96-L103)。GORM PostgreSQL driver 还会把裸 `timestamp` 的 `ScanLocation` 注册为 DSN location：`gorm.io/driver/postgres@v1.6.0/postgres.go:95-121`，因此读回 Go 时会把墙上时间解释为 `Asia/Shanghai`，但这是读取解释，不是列内元数据。
3. 当前本地 [config.yaml L3-L6](../config.yaml#L3-L6) 选择 PostgreSQL。因此当前配置实际采用的是“Asia/Shanghai session/扫描 location + 不带时区列”。这能让 `CURRENT_TIMESTAMP` 转为裸时间时较稳定，也与统计代码的注释相符（[admin_statistics_service.go L151-L163](../app/service/admin_statistics_service.go#L151-L163)），但不能反推出每条记录原始时区；数据库默认时间与显式 Go 参数还可能走不同转换路径。

### MySQL

1. `go-web@v1.8.4/pkg/server/database/config/config.go:62-86` 设置 `ParseTime=true`、`Loc=time.Local`，但没有设置 `Params["time_zone"]`。仓库安装测试生成的 DSN 也是 `parseTime=True&loc=Local`，[install_service.go L85-L94](../modules/identity/infrastructure/install_service.go#L85-L94)。
2. 锁定的 `go-sql-driver/mysql@v1.10.0` 源码在写入时先执行 `v.In(cfg.Loc)`，再编码不带 offset 的日期时间字符串；读取时用 `cfg.Loc` 解析：`connection.go:375-384`、`packets.go:869-877,1201-1215`。其 README 也明确指出 `loc` 只设置 Go `time.Time` 的 location，**不会改变 MySQL `time_zone`**：`README.md:296-306`。
3. 所以应用进程时区与 MySQL session/server 时区如果不同，`TIMESTAMP` 的墙上时间转换可能产生偏差；即使二者一致，原始 IANA 时区/offset 也没有逐行保存。

### SQLite

1. 运行时只把文件路径传给 SQLite，没有设置时区 DSN，见 `go-web@v1.8.4/pkg/server/database/driver/sqlite_driver.go:11-18`。本项目 SQLite 迁移统一声明 `DATETIME` / `DATE`。
2. `github.com/glebarez/go-sqlite@v1.22.0/sqlite.go:295-305,342-354,1135-1137` 显示：显式传入的 `time.Time` 会绑定为文本，默认格式是 `2006-01-02 15:04:05.999999999-07:00`，包含数值 offset；读取也接受带或不带 offset 的多种文本格式。
3. 因此 SQLite 的“列类型不记录时区”和“某些文本值含 offset”可以同时为真。它只保留写入当时的数值 offset，不保存 `Asia/Shanghai` 这种地区规则；而 SQL 默认值、数据迁移或其他客户端可能写入不同格式，列内语义并不受 schema 统一保证。

## 应用写入与读取路径

- GORM 默认 `NowFunc` 是 `time.Now().Local()`：`gorm.io/gorm@v1.31.1/gorm.go:180-182`。字段名为 `CreatedAt` / `UpdatedAt` 时会被识别为自动时间字段：`gorm.io/gorm@v1.31.1/schema/field.go:292-313`；软删除也用当前 `NowFunc` 写入：`soft_delete.go:142-146`。
- Docker 构建阶段和运行镜像都把系统时区设为 `Asia/Shanghai`（[Dockerfile L25-L30](../Dockerfile#L25-L30)、[L61-L66](../Dockerfile#L61-L66)）；原生 `make dev` 没有代码级 `TZ` 固定，因此 `time.Local` 仍依赖宿主环境。
- 大部分模型同时声明 `default:CURRENT_TIMESTAMP`，数据库可在零值插入时提供默认时间；应用也有许多显式值。例如消息任务直接写入 `time.Now()` 与请求中的 `ScheduledAt`（[service.go L115-L136](../modules/messaging/infrastructure/service.go#L115-L136)），`ScheduledAt` DTO 是 `*time.Time`（[message_dto.go L5-L22](../app/dto/message_dto.go#L5-L22)）。
- 终态事务把 `time.Now()` 写入 `updated_at`，并可写 provider 回调传入的 `callback_time`（[task_terminal_service.go L62-L87](../app/service/task_terminal_service.go#L62-L87)）；上行回调的 `CreatedAt` 也可来自 provider 接收时间（[task_terminal_service.go L116-L134](../app/service/task_terminal_service.go#L116-L134)）。因此外部 offset 进入 Go 后，在 PostgreSQL/MySQL 会丢失原始 offset，在 SQLite 显式写入路径上可能保留下来。
- Webhook outbox 的 `next_attempt_at`、`locked_until`、`updated_at` 均直接传递 Go `time.Time`（[webhook_log_dao.go L85-L124](../app/dao/webhook_log_dao.go#L85-L124)、[L151-L174](../app/dao/webhook_log_dao.go#L151-L174)）；比较正确性依赖同一方言内一致的解释规则，而不是列内保存的时区。
- 仍有查询直接使用数据库 session 时间或日期，例如 `scheduled_at <= NOW()`（[push_task_dao.go L108-L114](../app/dao/push_task_dao.go#L108-L114)）及 `DATE(created_at)`（[push_task_dao.go L220-L224](../app/dao/push_task_dao.go#L220-L224)、[push_batch_task_dao.go L73-L77](../app/dao/push_batch_task_dao.go#L73-L77)）。PostgreSQL session 已固定上海；MySQL session 未在当前客户端源码中固定，因此这些边界可能与进程 `time.Local` 不一致。
- 配额统计日先按进程本地 `time.Now()` 取 `YYYY-MM-DD`，再解析为 `time.Time` 写入 `DATE`（[quota_syncer.go L100-L126](../modules/delivery/infrastructure/scheduler/quota_syncer.go#L100-L126)）。落库只保存日期，不保存产生该日期时采用的时区。
- 多个短信供应商把不带 offset 的回执字符串按 `time.Local` 解析后写库，例如阿里云（[aliyun_sms_sender.go L450](../modules/sender/infrastructure/aliyun_sms_sender.go#L450)、[L501](../modules/sender/infrastructure/aliyun_sms_sender.go#L501)）、腾讯云（[tencent_sms_sender.go L594](../modules/sender/infrastructure/tencent_sms_sender.go#L594)）和网易（[netease_sms_sender.go L566-L587](../modules/sender/infrastructure/netease_sms_sender.go#L566-L587)）；SMTP 的 RFC 3339 路径则可解析输入 offset（[smtp_sender.go L564-L566](../modules/sender/infrastructure/smtp_sender.go#L564-L566)）。这些输入一旦进入 PostgreSQL/MySQL 裸列，原始 offset 均不可恢复。
- 多处管理 API 使用 `Format(time.RFC3339)` 输出，例如任务 [admin_task_service.go L193-L198](../app/service/admin_task_service.go#L193-L198) 和日志 [admin_log_service.go L231-L239](../app/service/admin_log_service.go#L231-L239)。输出中的 offset 是扫描后的 Go location；PostgreSQL/MySQL 已无法证明它等于原始写入 offset。另有失败规则接口使用不带 offset 的格式 `2006-01-02 15:04:05`（[failure_rule_controller.go L265-L266](../app/controller/admin/failure_rule_controller.go#L265-L266)）。

## 与时区无关但名称含 time/timeout 的列

以下字段表达时长而非日期/时间点，不应纳入“是否记录时区”的 52 列计数：

| 表.字段 | 类型/单位 | 模型证据 |
|---|---|---|
| `push_logs.cost_time` | `INT`，毫秒 | [push_log.go L18](../app/model/push_log.go#L18) |
| `channel_health_history.response_time` | `INT`，毫秒 | [channel_health_history.go L13](../app/model/channel_health_history.go#L13) |
| `webhook_configs.timeout` | `INT`，秒 | [webhook_config.go L16](../app/model/webhook_config.go#L16) |
| `webhook_logs.timeout_seconds` | `INT`，秒 | [webhook_log.go L24](../app/model/webhook_log.go#L24) |

## 风险归纳

1. **跨方言行为不一致**：同一 Go `time.Time` 的 offset 在 SQLite 文本中可能保留，在 PostgreSQL/MySQL 中不会保留。
2. **MySQL 双时区来源**：应用 `time.Local` 与数据库 session `time_zone` 没有在当前源码中被绑定为同一值。
3. **PostgreSQL 绝对时间歧义**：固定 session 为 `Asia/Shanghai` 降低了当前部署中的歧义，但裸 `TIMESTAMP` 仍不能自描述；换时区读取或迁移数据时需要额外约定。
4. **API offset 不等于原始 offset**：RFC 3339 输出看起来“有时区”，但可能只是读取后附加的 location。
5. **SQLite 列内格式可能混杂**：显式 Go 写入、数据库默认值、迁移复制值的文本来源不同，不能只凭列名/声明假设所有历史值格式一致。

所以，对问题“现在所有表的时间有关字段是否都没有记录时区？”最准确的简答是：**schema 层面是，全部 52 个日期/时间列都没有可靠记录每行原始时区；但 SQLite 显式写入的部分文本值可能携带数值 offset，PostgreSQL 连接也有固定 session 时区，这两者都不能等同于列记录了时区。**
