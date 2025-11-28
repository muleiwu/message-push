# 木雷消息服务 - 完整API使用指南

## 目录

1. [快速开始](#快速开始)
2. [管理后台配置](#管理后台配置)
3. [发送消息](#发送消息)
4. [查询任务](#查询任务)
5. [统计分析](#统计分析)
6. [最佳实践](#最佳实践)

## 快速开始

### 第一步：启动服务

```bash
# 设置加密密钥
export APP_ENCRYPTION_KEY="your-32-byte-encryption-key-!!"

# 启动服务
go run main.go
```

服务启动后监听在 `http://localhost:8080`

### 第二步：创建应用

```bash
curl -X POST http://localhost:8080/api/admin/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的应用",
    "status": 1
  }'
```

**响应示例**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "app_key": "a1b2c3d4e5f67890",
    "app_secret": "secret1234567890abcdef",
    "daily_limit": 10000,
    "qps_limit": 100
  }
}
```

保存 `app_key` 和 `app_secret`，后续发送消息时需要使用。

## 管理后台配置

### 1. 创建服务商

```bash
curl -X POST http://localhost:8080/api/admin/providers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "阿里云SMS",
    "type": "sms",
    "description": "阿里云短信服务",
    "config": {
      "access_key_id": "your_aliyun_key",
      "access_key_secret": "your_aliyun_secret",
      "sign_name": "我的签名",
      "region": "cn-hangzhou"
    },
    "status": 1
  }'
```

### 2. 创建通道

```bash
curl -X POST http://localhost:8080/api/admin/channels \
  -H "Content-Type: application/json" \
  -d '{
    "name": "短信通道",
    "type": "sms",
    "status": 1
  }'
```

### 3. 绑定服务商到通道

```bash
curl -X POST http://localhost:8080/api/admin/channels/bind-provider \
  -H "Content-Type: application/json" \
  -d '{
    "channel_id": 1,
    "provider_id": 1,
    "priority": 1,
    "weight": 10
  }'
```

## 发送消息

### 签名生成

所有发送消息的请求都需要签名认证。签名算法：

```
signature = HMAC-SHA256(app_secret, "app_key={app_key}&timestamp={timestamp}")
```

### Python 签名示例

```python
import hmac
import hashlib
import time
import json
import requests

app_key = "your_app_key"
app_secret = "your_app_secret"
timestamp = str(int(time.time()))

# 生成签名
sign_string = f"app_key={app_key}&timestamp={timestamp}"
signature = hmac.new(
    app_secret.encode('utf-8'),
    sign_string.encode('utf-8'),
    hashlib.sha256
).hexdigest()

# 构建请求头
headers = {
    "Content-Type": "application/json",
    "X-App-Key": app_key,
    "X-Timestamp": timestamp,
    "X-Signature": signature
}

# 发送消息
data = {
    "channel_type": "sms",
    "receiver": "13800138000",
    "content": "您的验证码是123456",
    "params": {
        "code": "123456"
    }
}

response = requests.post(
    "http://localhost:8080/api/v1/messages",
    headers=headers,
    json=data
)

print(response.json())
```

### Go 签名示例

```go
package main

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"
)

func generateSignature(appKey, appSecret string, timestamp string) string {
    signString := fmt.Sprintf("app_key=%s&timestamp=%s", appKey, timestamp)
    h := hmac.New(sha256.New, []byte(appSecret))
    h.Write([]byte(signString))
    return hex.EncodeToString(h.Sum(nil))
}

func main() {
    appKey := "your_app_key"
    appSecret := "your_app_secret"
    timestamp := fmt.Sprintf("%d", time.Now().Unix())
    
    signature := generateSignature(appKey, appSecret, timestamp)
    fmt.Printf("Signature: %s\n", signature)
}
```

### 单条消息发送

```bash
# 发送短信
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "sms",
    "receiver": "13800138000",
    "content": "您的验证码是123456",
    "params": {
      "code": "123456"
    }
  }'
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "task_abc123456"
  }
}
```

### 批量消息发送

```bash
curl -X POST http://localhost:8080/api/v1/messages/batch \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "sms",
    "receivers": [
      "13800138000",
      "13800138001",
      "13800138002"
    ],
    "content": "活动通知：限时优惠进行中",
    "params": {
      "activity_name": "双十一"
    }
  }'
```

### 定时消息发送

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "sms",
    "receiver": "13800138000",
    "content": "会议提醒：明天上午10点开会",
    "scheduled_at": "2025-11-20T10:00:00+08:00"
  }'
```

### Email 发送示例

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature" \
  -d '{
    "channel_type": "email",
    "receiver": "user@example.com",
    "subject": "账户验证",
    "content": "您的验证码是: 123456",
    "params": {
      "code": "123456"
    }
  }'
```

## 查询任务

### 查询单个任务

```bash
curl -X GET http://localhost:8080/api/v1/messages/task_abc123456 \
  -H "X-App-Key: your_app_key" \
  -H "X-Timestamp: 1700000000" \
  -H "X-Signature: your_signature"
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "task_abc123456",
    "status": "success",
    "channel_type": "sms",
    "receiver": "13800138000",
    "created_at": "2025-11-19T10:00:00Z",
    "sent_at": "2025-11-19T10:00:01Z"
  }
}
```

任务状态说明：
- `pending`: 等待发送
- `processing`: 发送中
- `success`: 发送成功
- `failed`: 发送失败
- `scheduled`: 定时任务等待中

## 统计分析

### 获取统计数据

```bash
curl -X GET "http://localhost:8080/api/admin/statistics?start_date=2025-11-01&end_date=2025-11-19&app_id=1" \
  -H "Content-Type: application/json"
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "summary": {
      "total_count": 10000,
      "success_count": 9500,
      "failure_count": 500,
      "success_rate": "95.00%"
    },
    "daily": [
      {
        "date": "2025-11-01",
        "total_count": 500,
        "success_count": 475,
        "failure_count": 25,
        "success_rate": "95.00%"
      }
    ]
  }
}
```

### 获取应用配额使用情况

```bash
curl -X GET http://localhost:8080/api/admin/applications/1 \
  -H "Content-Type: application/json"
```

## 最佳实践

### 1. 错误处理

```python
def send_message(receiver, content):
    try:
        response = requests.post(url, headers=headers, json=data, timeout=10)
        result = response.json()
        
        if result['code'] == 0:
            task_id = result['data']['task_id']
            print(f"发送成功，任务ID: {task_id}")
            return task_id
        else:
            print(f"发送失败: {result['message']}")
            return None
    except requests.Timeout:
        print("请求超时，请重试")
    except Exception as e:
        print(f"发生错误: {e}")
```

### 2. 重试机制

服务端已内置重试机制（最多3次），客户端无需额外实现。

### 3. 批量发送优化

```python
# 推荐：每批次100-500条
receivers = ["13800138000", "13800138001", ...]

# 分批发送
batch_size = 500
for i in range(0, len(receivers), batch_size):
    batch = receivers[i:i+batch_size]
    send_batch_message(batch, content)
    time.sleep(1)  # 避免触发限流
```

### 4. 限流处理

当返回限流错误时：
```json
{
  "code": 429,
  "message": "rate limit exceeded"
}
```

建议实现指数退避策略：
```python
import time

def send_with_retry(data, max_retries=3):
    for i in range(max_retries):
        response = send_message(data)
        if response['code'] == 0:
            return response
        elif response['code'] == 429:
            wait_time = 2 ** i  # 1s, 2s, 4s
            time.sleep(wait_time)
        else:
            break
    return None
```

### 5. 监控告警

建议监控以下指标：
- 发送成功率（目标 > 95%）
- 平均延迟（目标 < 500ms）
- 每日配额使用率（告警阈值 80%）
- QPS峰值

```bash
# 每小时检查一次统计数据
*/60 * * * * curl http://localhost:8080/api/admin/statistics?start_date=$(date +%Y-%m-%d)&end_date=$(date +%Y-%m-%d)
```

### 6. 安全建议

1. **保护密钥**：不要在代码中硬编码 app_secret
```python
import os
app_secret = os.environ.get('APP_SECRET')
```

2. **HTTPS**：生产环境务必使用 HTTPS
3. **IP白名单**：在应用配置中设置 IP 白名单
4. **定期轮换密钥**：建议每季度更换一次 app_secret

### 7. 性能优化

1. **使用连接池**
```python
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

session = requests.Session()
adapter = HTTPAdapter(pool_connections=10, pool_maxsize=100)
session.mount('http://', adapter)
```

2. **异步发送**
```python
import asyncio
import aiohttp

async def send_message_async(data):
    async with aiohttp.ClientSession() as session:
        async with session.post(url, json=data, headers=headers) as response:
            return await response.json()
```

## 错误码参考

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 0 | 成功 | - |
| 400 | 请求参数错误 | 检查请求参数格式 |
| 401 | 签名验证失败 | 检查 app_key/app_secret/签名算法 |
| 403 | 应用被禁用 | 联系管理员 |
| 404 | 任务不存在 | 检查 task_id |
| 429 | 超过速率限制 | 降低发送频率或联系管理员提升限额 |
| 500 | 服务器内部错误 | 联系技术支持 |
| 503 | 服务暂时不可用 | 稍后重试 |

## 常见问题

### Q: 如何提高发送成功率？

A: 
1. 确保接收号码格式正确
2. 内容符合运营商规范（避免敏感词）
3. 选择合适的服务商（不同服务商成功率不同）
4. 合理配置通道负载均衡

### Q: 消息延迟多久发送？

A: 
- 即时消息：< 1秒
- 定时消息：按设定时间发送（误差 < 10秒）

### Q: 支持国际短信吗？

A: 支持，需要配置支持国际短信的服务商。

### Q: 如何查看发送失败原因？

A: 通过统计查询API查看详细的失败日志。

## 技术支持

- 文档：[API_GUIDE.md](API_GUIDE.md)
- 快速开始：[QUICKSTART.md](QUICKSTART.md)
- Issue：提交到项目 Issue 页面

---

**更新日期**: 2025-11-19  
**版本**: v0.95.0

