# 生产环境部署指南

## 概述

本文档详细说明木雷消息服务在生产环境的部署步骤和最佳实践。

## 目录

1. [部署前准备](#部署前准备)
2. [Docker部署](#docker部署)
3. [Kubernetes部署](#kubernetes部署)
4. [数据库准备](#数据库准备)
5. [监控告警](#监控告警)
6. [性能调优](#性能调优)
7. [故障排查](#故障排查)

## 部署前准备

### 1. 硬件要求

#### 最小配置（单实例）
- CPU: 2核
- 内存: 4GB
- 磁盘: 20GB SSD

#### 推荐配置（生产环境）
- CPU: 4核+
- 内存: 8GB+
- 磁盘: 100GB SSD+
- 网络: 千兆网卡

### 2. 软件依赖

- Docker: 20.10+
- Kubernetes: 1.20+ (可选)
- MySQL: 8.0+
- Redis: 6.0+

### 3. 网络要求

- 应用端口: 8080
- MySQL端口: 3306
- Redis端口: 6379
- 网络延迟: < 10ms (应用与数据库之间)

## Docker部署

### 1. 构建镜像

```bash
# 创建Dockerfile
cat > Dockerfile <<'EOF'
FROM golang:1.23-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git make

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o message-push ./main.go

# 最终镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 设置时区
ENV TZ=Asia/Shanghai

# 复制编译好的二进制文件
COPY --from=builder /app/message-push .
COPY --from=builder /app/config.yaml.example ./config.yaml

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=40s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/health/simple || exit 1

# 启动命令
CMD ["./message-push"]
EOF

# 构建镜像
docker build -t message-push:v0.95.0 .
```

### 2. 使用Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  message-push:
    image: message-push:v0.95.0
    container_name: message-push
    ports:
      - "8080:8080"
    environment:
      - APP_ENCRYPTION_KEY=your-32-byte-encryption-key-!!
      - MYSQL_HOST=mysql
      - MYSQL_PORT=3306
      - MYSQL_DATABASE=push_service
      - MYSQL_USERNAME=root
      - MYSQL_PASSWORD=your_password
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    depends_on:
      - mysql
      - redis
    restart: unless-stopped
    networks:
      - push-network
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health/simple"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 40s

  mysql:
    image: mysql:8.0
    container_name: message-push-mysql
    environment:
      - MYSQL_ROOT_PASSWORD=your_password
      - MYSQL_DATABASE=push_service
      - MYSQL_CHARACTER_SET_SERVER=utf8mb4
      - MYSQL_COLLATION_SERVER=utf8mb4_unicode_ci
    volumes:
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"
    networks:
      - push-network
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    container_name: message-push-redis
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    networks:
      - push-network
    restart: unless-stopped
    command: redis-server --appendonly yes

volumes:
  mysql-data:
  redis-data:

networks:
  push-network:
    driver: bridge
```

### 3. 启动服务

```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f message-push

# 检查状态
docker-compose ps

# 停止服务
docker-compose down
```

## Kubernetes部署

### 1. 准备命名空间

```bash
# 创建命名空间
kubectl create namespace message-push

# 设置默认命名空间
kubectl config set-context --current --namespace=message-push
```

### 2. 创建Secret

```bash
# 生成加密密钥
ENCRYPTION_KEY=$(openssl rand -base64 32)

# 创建Secret
kubectl create secret generic message-push-secret \
  --from-literal=encryption-key="$ENCRYPTION_KEY" \
  --from-literal=mysql-username="root" \
  --from-literal=mysql-password="your_mysql_password" \
  --from-literal=redis-password="" \
  -n message-push
```

### 3. 应用配置

```bash
# 应用所有配置
kubectl apply -f deploy/kubernetes/deployment.yaml -n message-push

# 查看部署状态
kubectl get pods -n message-push -w

# 查看服务
kubectl get svc -n message-push

# 查看日志
kubectl logs -f deployment/message-push -n message-push
```

### 4. 配置Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: message-push-ingress
  namespace: message-push
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "600"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
spec:
  tls:
  - hosts:
    - push.example.com
    secretName: message-push-tls
  rules:
  - host: push.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: message-push
            port:
              number: 8080
```

### 5. 水平扩展

```bash
# 手动扩容
kubectl scale deployment message-push --replicas=5 -n message-push

# 查看HPA状态
kubectl get hpa -n message-push

# 查看扩容事件
kubectl describe hpa message-push-hpa -n message-push
```

## 数据库准备

### 1. MySQL初始化

```sql
-- 创建数据库
CREATE DATABASE IF NOT EXISTS push_service 
  DEFAULT CHARACTER SET utf8mb4 
  DEFAULT COLLATE utf8mb4_unicode_ci;

-- 创建用户
CREATE USER 'push_user'@'%' IDENTIFIED BY 'your_strong_password';
GRANT ALL PRIVILEGES ON push_service.* TO 'push_user'@'%';
FLUSH PRIVILEGES;

-- 优化配置 (my.cnf)
[mysqld]
max_connections = 1000
innodb_buffer_pool_size = 4G
innodb_log_file_size = 512M
innodb_flush_log_at_trx_commit = 2
innodb_flush_method = O_DIRECT
```

### 2. 数据库迁移

```bash
# 自动迁移（首次启动时）
./message-push

# 或使用迁移工具
go run cmd/migrate/main.go
```

### 3. Redis配置

```conf
# redis.conf
maxmemory 2gb
maxmemory-policy allkeys-lru
appendonly yes
appendfsync everysec

# 持久化配置
save 900 1
save 300 10
save 60 10000
```

## 监控告警

### 1. Prometheus配置

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'message-push'
    static_configs:
      - targets: ['message-push:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### 2. Grafana Dashboard

导入预定义的Dashboard：

- 系统指标：CPU、内存、网络
- 应用指标：QPS、延迟、错误率
- 业务指标：发送量、成功率、队列长度

### 3. 告警规则

```yaml
# alert-rules.yaml
groups:
  - name: message-push
    rules:
      - alert: HighErrorRate
        expr: rate(push_errors_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "推送错误率过高"
          description: "错误率超过5%"
          
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(push_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "推送延迟过高"
          description: "P99延迟超过1秒"
          
      - alert: QueueBacklog
        expr: redis_stream_length{stream="push:stream:messages"} > 10000
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "消息队列积压"
          description: "队列长度超过10000"
```

## 性能调优

### 1. 应用配置

```yaml
# config.yaml (生产环境)
database:
  max_idle_conns: 50
  max_open_conns: 200
  conn_max_lifetime: 3600

redis:
  pool_size: 100
  min_idle_conns: 10

worker:
  pool_size: 20  # 根据CPU核心数调整

queue:
  batch_size: 50  # 增加批量大小
  block_time: 3000
```

### 2. 系统参数

```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 10000 65000
net.core.netdev_max_backlog = 4096

# 应用
sysctl -p
```

### 3. 容器资源限制

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

## 备份与恢复

### 1. 数据库备份

```bash
# 每日备份脚本
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backup/mysql"

mysqldump -h mysql-host -u root -p'password' \
  --single-transaction \
  --routines \
  --triggers \
  --databases push_service \
  | gzip > $BACKUP_DIR/push_service_$DATE.sql.gz

# 保留最近7天的备份
find $BACKUP_DIR -name "push_service_*.sql.gz" -mtime +7 -delete
```

### 2. Redis备份

```bash
# Redis持久化备份
redis-cli --rdb /backup/redis/dump_$(date +%Y%m%d).rdb

# 保留最近3天
find /backup/redis -name "dump_*.rdb" -mtime +3 -delete
```

## 安全加固

### 1. 网络安全

```bash
# 防火墙规则
ufw allow 8080/tcp
ufw allow from 10.0.0.0/8 to any port 3306
ufw allow from 10.0.0.0/8 to any port 6379
ufw enable
```

### 2. 密钥管理

- 使用Kubernetes Secrets或外部密钥管理服务
- 定期轮换密钥
- 最小权限原则

### 3. HTTPS配置

```bash
# 使用Let's Encrypt
cert-manager install
kubectl apply -f ingress-tls.yaml
```

## 故障排查

### 1. 常见问题

#### 服务无法启动

```bash
# 检查日志
kubectl logs deployment/message-push -n message-push --tail=100

# 检查配置
kubectl describe pod <pod-name> -n message-push

# 检查资源
kubectl top pods -n message-push
```

#### 数据库连接失败

```bash
# 测试连接
mysql -h mysql-host -u root -p

# 检查网络
kubectl exec -it <pod-name> -n message-push -- ping mysql-service
```

#### Redis连接超时

```bash
# 测试Redis
redis-cli -h redis-host ping

# 检查Redis状态
redis-cli info
```

### 2. 性能问题

```bash
# 查看慢查询
mysql> SELECT * FROM mysql.slow_log ORDER BY start_time DESC LIMIT 10;

# Redis慢日志
redis-cli slowlog get 10

# 应用性能分析
go tool pprof http://localhost:8080/debug/pprof/profile
```

## 滚动更新

### 1. 无停机更新

```bash
# 更新镜像
kubectl set image deployment/message-push \
  message-push=message-push:v0.96.0 \
  -n message-push

# 查看更新状态
kubectl rollout status deployment/message-push -n message-push

# 回滚
kubectl rollout undo deployment/message-push -n message-push
```

### 2. 蓝绿部署

```bash
# 部署新版本
kubectl apply -f deployment-v096.yaml

# 切换流量
kubectl patch service message-push -p '{"spec":{"selector":{"version":"v0.96.0"}}}'

# 验证后删除旧版本
kubectl delete deployment message-push-v095
```

## 运维检查清单

### 每日检查
- [ ] 服务健康状态
- [ ] 错误日志
- [ ] 发送成功率
- [ ] 队列积压情况

### 每周检查
- [ ] 数据库备份验证
- [ ] 磁盘空间
- [ ] 性能指标趋势
- [ ] 安全漏洞扫描

### 每月检查
- [ ] 密钥轮换
- [ ] 容量规划
- [ ] 成本优化
- [ ] 文档更新

## 参考资料

- [Kubernetes官方文档](https://kubernetes.io/docs/)
- [Docker最佳实践](https://docs.docker.com/develop/dev-best-practices/)
- [MySQL性能优化](https://dev.mysql.com/doc/refman/8.0/en/optimization.html)
- [Redis生产部署](https://redis.io/topics/admin)

---

**文档版本**: v1.0  
**更新日期**: 2025-11-19  
**适用版本**: v0.95.0+
