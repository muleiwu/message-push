# UTC 时间迁移发布手册

## 数据口径

- 所有时间点统一按 UTC 绝对时刻存储、比较和通过 API 返回。
- 升级前没有 offset 的历史时间按 UTC 解释，不按旧连接的 `Asia/Shanghai` 语义换算。
- `app_quota_stats.stat_date`、`provider_quota_stats.stat_date` 仍是上海业务日期；时长字段不迁移。
- 报表自然日固定为 `Asia/Shanghai`，与数据库 session 和进程默认时区无关。

## 发布前

1. 完成数据库全量备份，并验证备份可恢复。
2. 进入维护窗口，停止 API 写入、worker 和 scheduler；确认没有运行中的消息投递或 Webhook 租约更新。
3. 运行快速测试与三方言集成测试：

   ```bash
   make test
   make test-timezone-integration
   ```

## 升级

启动新版本。数据库迁移是服务启动序列的第一项业务服务；任何迁移失败都会阻止 worker、scheduler 和 HTTP 服务继续启动。不要绕过失败后强制恢复流量。

## 升级后检查

PostgreSQL：确认 50 个时间点列均为 `timestamp with time zone`，并抽样检查历史值按 UTC 保留。

```sql
SELECT data_type, COUNT(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND data_type LIKE 'timestamp%'
GROUP BY data_type;

SHOW TIME ZONE;
```

MySQL：确认应用连接的 session 为 UTC，并抽样检查跨 offset 写入读取为同一时刻。

```sql
SELECT @@session.time_zone;
```

SQLite：抽样确认时间点文本带 `+00:00`，历史带 offset 值已经换算为 UTC；`stat_date` 未改变。

确认 API 的任务、日志、回调和失败规则时间均为 RFC3339 `Z` 输出，统计响应仍返回 `timezone: "Asia/Shanghai"`。检查无误后再恢复流量。

## 回滚

- PostgreSQL Down 会把时间点统一还原成 UTC 裸 `timestamp`。
- MySQL 此版本迁移只同步 Goose 版本，列类型不变。
- SQLite UTC 文本规范化不可恢复原 offset；必须使用发布前备份回滚。
- 回滚期间同样保持所有写入、worker 和 scheduler 停止。
