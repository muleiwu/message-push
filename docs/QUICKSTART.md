# 消息推送服务 - 快速启动指南

## 🚀 5分钟快速体验

### 前置条件

- Go 1.21+
- MySQL 5.7+
- Redis 5.0+
- 可选: jq (用于测试脚本)

### 1. 准备配置文件

```bash
# 复制配置示例
cp config.yaml.example config.yaml

# 编辑配置（根据实际环境修改）
vim config.yaml
```

关键配置：
```yaml
database:
  host: localhost
  port: 3306
  database: push_service
  username: root
  password: your_password

redis:
  host: localhost
  port: 6379
```

### 2. 创建数据库

```sql
CREATE DATABASE push_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 3. 执行数据库迁移

```bash
# 方式1: 使用 Makefile
make migrate-up

# 方式2: 直接运行
go run cmd/migrate/main.go -action=up

# 填充测试数据
go run cmd/migrate/main.go -action=seed
```

迁移成功后，会创建以下表：
- applications (应用管理)
- providers (服务商)
- provider_channels (服务商通道)
- push_channels (业务通道)
- channel_provider_relations (通道关联)
- push_tasks (推送任务)
- push_batch_tasks (批量任务)
- push_logs (推送日志)
- channel_health_history (健康检查历史)
- app_quota_stats & provider_quota_stats (配额统计)

测试数据包括：
- 应用: test_app_001 / test_secret_please_change_in_production
- 服务商: aliyun_sms (阿里云短信)

### 4. 启动服务

```bash
# 方式1: 直接运行
go run main.go

# 方式2: 编译后运行
go build -o bin/push-service main.go
./bin/push-service
```

服务启动后会：
1. 初始化配置和数据库连接
2. 启动 Worker Pool (10个worker)
3. 启动 HTTP 服务器 (默认端口 8080)

### 5. 测试 API

#### 方式1: 使用测试脚本

```bash
./scripts/test_api.sh
```

#### 方式2: 手动测试

**健康检查：**
```bash
curl http://localhost:8080/health
```

**发送消息（需要认证）：**
```bash
# 注意：签名验证暂未完整实现，可临时跳过
curl -X POST http://localhost:8080/api/v1/messages \
  -H "X-App-Id: test_app_001" \
  -H "X-Signature: test_signature" \
  -H "X-Timestamp: $(date +%s)" \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": 1,
    "message_type": "sms",
    "receiver": "13800138000",
    "template_code": "verify_code",
    "template_params": {
      "code": "123456",
      "expire": "5"
    }
  }'
```

**查询任务状态：**
```bash
curl http://localhost:8080/api/v1/messages/{task_id} \
  -H "X-App-Id: test_app_001" \
  -H "X-Signature: test_signature" \
  -H "X-Timestamp: $(date +%s)"
```

## 📊 验证运行状态

### 检查日志

服务日志会输出：
```
worker pool started with 10 workers
worker started id=1
worker started id=2
...
HTTP server listening on: :8080
```

### 检查数据库

```sql
-- 查看任务表
SELECT * FROM push_tasks ORDER BY created_at DESC LIMIT 10;

-- 查看任务状态分布
SELECT status, COUNT(*) as count FROM push_tasks GROUP BY status;
```

### 检查Redis

```bash
# 查看队列长度
redis-cli XLEN push:stream:messages

# 查看消费者组信息
redis-cli XINFO GROUPS push:stream:messages

# 查看配额使用
redis-cli KEYS "quota:*"
```

## 🏗️ 架构说明

### 消息流转

```
1. 客户端发送请求 → Controller (认证/限流/配额)
2. MessageService创建任务 → 推送到Redis Stream
3. Worker从队列消费消息 → MessageHandler处理
4. ChannelSelector选择服务商通道 (平滑加权轮询)
5. Sender发送消息 (SMS/Email/企微/钉钉)
6. 更新任务状态 → 成功/失败反馈到熔断器
7. 失败自动重试 (指数退避)
```

### 中间件链

```
AuthMiddleware → RateLimitMiddleware → QuotaMiddleware → Controller
```

### 关键组件

- **Worker Pool**: 10个并发worker消费队列
- **Channel Selector**: 平滑加权轮询算法
- **Circuit Breaker**: 滑动窗口熔断器
- **Retry Helper**: 指数退避重试策略
- **Signature Helper**: HMAC-SHA256签名验证

## 🔧 常用命令

```bash
# 查看所有make命令
make help

# 数据库迁移
make migrate-up      # 执行迁移
make migrate-down    # 回滚迁移
make migrate-fresh   # 清空并重新迁移

# 构建
make build          # 构建二进制文件
make build-linux    # 交叉编译Linux版本

# 测试
make test           # 运行测试

# Docker
make docker-build   # 构建Docker镜像
make docker-run     # 运行Docker容器
```

## 🐛 故障排查

### 服务无法启动

1. 检查配置文件 config.yaml 是否存在
2. 确认数据库连接信息正确
3. 确认Redis服务运行中
4. 查看日志输出错误信息

### 消息发送失败

1. 检查任务表状态: `SELECT * FROM push_tasks WHERE status='failed'`
2. 查看Worker日志
3. 确认服务商配置正确
4. 检查熔断器状态

### Worker未消费消息

1. 确认Worker Pool已启动
2. 检查Redis Stream: `redis-cli XLEN push:stream:messages`
3. 查看消费者组: `redis-cli XINFO GROUPS push:stream:messages`
4. 检查死信队列: `redis-cli XLEN push:stream:dead_letter`

## 📝 下一步

1. **配置真实服务商**: 修改服务商配置，接入实际的阿里云/腾讯云SDK
2. **完善签名验证**: 实现AppSecret加密存储和验证
3. **添加定时任务**: 实现scheduled tasks扫描器
4. **开发管理后台**: 应用管理、通道管理、统计查询
5. **性能测试**: 压测并优化性能瓶颈

## 📚 相关文档

- [开发规范](PROJECT_SPECIFICATION.md)
- [API文档](API_GUIDE.md)
- [安装指南](INSTALL_GUIDE.md)
- [生产部署](PRODUCTION_DEPLOYMENT.md)

## 🆘 获取帮助

- 查看 docs/ 目录下的详细文档
- 查看代码注释和TODO标记
- 检查日志输出

---

**当前版本**: v0.9.0-beta  
**状态**: 核心功能完成，可用于开发测试环境  
**下一版本**: v1.0.0 (生产就绪)

