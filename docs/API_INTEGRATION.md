# 木雷消息服务 - API 对接文档

> 本文档面向外部开发者，详细说明如何对接木雷消息服务 API。

## 目录

1. [快速开始](#快速开始)
2. [认证与签名](#认证与签名)
3. [API 接口](#api-接口)
4. [多语言示例](#多语言示例)
5. [错误码参考](#错误码参考)
6. [附录](#附录)

---

## 快速开始

### 接入流程

1. 获取 `app_id` 和 `app_secret`（由管理员分配）
2. 按照签名算法生成请求签名
3. 调用 API 发送消息
4. 通过任务 ID 查询发送状态

### 基础信息

| 项目 | 说明 |
|------|------|
| 基础 URL | `https://your-domain.com` |
| 协议 | HTTPS（生产环境必须） |
| 数据格式 | JSON |
| 字符编码 | UTF-8 |

---

## 认证与签名

所有 API 请求都需要在 HTTP Header 中携带认证信息。

### 请求头

| Header | 必填 | 说明 |
|--------|------|------|
| `X-App-Id` | 是 | 应用 ID |
| `X-Signature` | 是 | 请求签名 |
| `X-Timestamp` | 是 | Unix 时间戳（秒） |
| `X-Nonce` | 是 | 随机字符串（用于防重放攻击） |
| `Content-Type` | 是 | `application/json` |

### 签名算法

签名采用 HMAC-SHA256 算法，步骤如下：

#### 步骤 1：构造签名内容

```
SignContent = Method + Path + SortedParams + Timestamp + Nonce
```

- `Method`: HTTP 方法（大写，如 `POST`、`GET`）
- `Path`: 请求路径（如 `/api/v1/messages`）
- `SortedParams`: 请求体 JSON 按 key 排序后的字符串（无请求体时为空字符串）
- `Timestamp`: Unix 时间戳（秒）
- `Nonce`: 随机字符串

#### 步骤 2：计算签名

```
Signature = Hex(HMAC-SHA256(SignContent, AppSecret))
```

- 使用 `AppSecret` 作为密钥
- 对 `SignContent` 进行 HMAC-SHA256 运算
- 结果进行十六进制编码（小写）

### 签名示例

假设：
- `app_secret` = `secret123456`
- `method` = `POST`
- `path` = `/api/v1/messages`
- `timestamp` = `1700000000`
- `nonce` = `abc123`
- `request_body` = `{"channel_id":1,"receiver":"13800138000","template_params":{"code":"123456"}}`

计算过程：
```
1. SortedParams = {"channel_id":1,"receiver":"13800138000","template_params":{"code":"123456"}}
2. SignContent = "POST/api/v1/messages{\"channel_id\":1,\"receiver\":\"13800138000\",\"template_params\":{\"code\":\"123456\"}}1700000000abc123"
3. Signature = Hex(HMAC-SHA256(SignContent, "secret123456"))
```

### 安全说明

- **时间戳有效期**：服务端校验时间戳与当前时间差不超过 ±5 分钟，防止重放攻击
- **Nonce 唯一性**：建议使用 UUID 或随机字符串，确保每次请求唯一
- **密钥保护**：`app_secret` 应妥善保管，不要在前端代码或公开环境中暴露
- **HTTPS**：生产环境务必使用 HTTPS 协议

---

## API 接口

### 1. 发送消息

发送单条消息。

**请求**

```
POST /api/v1/messages
```

**请求参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel_id` | int | 是 | 通道 ID（通道绑定了消息类型和模板） |
| `receiver` | string | 是 | 接收者（手机号/邮箱/用户ID） |
| `template_params` | object | 否 | 模板参数（键值对） |
| `signature_name` | string | 否 | 签名名称（用于 SMS 消息，如"公司名称"） |
| `scheduled_at` | string | 否 | 定时发送时间（ISO 8601 格式） |

**请求示例**

```json
{
  "channel_id": 1,
  "receiver": "13800138000",
  "template_params": {
    "code": "123456",
    "expire_time": "5"
  },
  "signature_name": "公司名称"
}
```

**响应参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `code` | int | 状态码，0 表示成功 |
| `message` | string | 状态描述 |
| `data.task_id` | string | 任务 ID（UUID 格式） |
| `data.status` | string | 任务状态 |
| `data.created_at` | string | 创建时间 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "created_at": "2025-11-25T10:00:00Z"
  }
}
```

---

### 2. 批量发送消息

批量发送消息给多个接收者（所有接收者共用相同的模板参数）。

**请求**

```
POST /api/v1/messages/batch
```

**请求参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel_id` | int | 是 | 通道 ID（通道绑定了消息类型和模板） |
| `receivers` | string[] | 是 | 接收者列表（手机号/邮箱/用户ID数组） |
| `template_params` | object | 否 | 模板参数（所有接收者共用） |
| `signature_name` | string | 否 | 签名名称（用于 SMS 消息，如"公司名称"） |
| `scheduled_at` | string | 否 | 定时发送时间（ISO 8601 格式） |

**请求示例**

```json
{
  "channel_id": 1,
  "receivers": [
    "13800138000",
    "13800138001",
    "13800138002"
  ],
  "template_params": {
    "content": "系统将于今晚22:00进行维护",
    "duration": "2小时"
  },
  "signature_name": "公司名称"
}
```

**响应参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `code` | int | 状态码 |
| `message` | string | 状态描述 |
| `data.batch_id` | string | 批次 ID |
| `data.total_count` | int | 总数量 |
| `data.success_count` | int | 成功入队数量 |
| `data.failed_count` | int | 失败数量 |
| `data.created_at` | string | 创建时间 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "batch_id": "660e8400-e29b-41d4-a716-446655440001",
    "total_count": 3,
    "success_count": 3,
    "failed_count": 0,
    "created_at": "2025-11-25T10:00:00Z"
  }
}
```

---

### 3. 查询任务状态

根据任务 ID 查询发送状态。

**请求**

```
GET /api/v1/messages/{task_id}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `task_id` | string | 是 | 任务 ID |

**响应参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `code` | int | 状态码 |
| `message` | string | 状态描述 |
| `data.task_id` | string | 任务 ID |
| `data.app_id` | string | 应用 ID |
| `data.channel_id` | int | 通道 ID |
| `data.message_type` | string | 消息类型 |
| `data.receiver` | string | 接收者 |
| `data.content` | string | 消息内容 |
| `data.status` | string | 任务状态 |
| `data.callback_status` | string | 回调状态（如有） |
| `data.retry_count` | int | 已重试次数 |
| `data.created_at` | string | 创建时间 |
| `data.updated_at` | string | 更新时间 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 1,
    "task_id": "550e8400-e29b-41d4-a716-446655440000",
    "app_id": "test_app_001",
    "channel_id": 1,
    "message_type": "sms",
    "receiver": "13800138000",
    "content": "您的验证码是123456，5分钟内有效。",
    "status": "success",
    "callback_status": "delivered",
    "retry_count": 0,
    "max_retry": 3,
    "created_at": "2025-11-25T10:00:00Z",
    "updated_at": "2025-11-25T10:00:02Z"
  }
}
```

---

## 多语言示例

### Python

```python
import hmac
import hashlib
import time
import json
import uuid
import requests

class MessagePushClient:
    def __init__(self, base_url, app_id, app_secret):
        self.base_url = base_url
        self.app_id = app_id
        self.app_secret = app_secret

    def _sort_params(self, params):
        """按 key 排序参数"""
        if not params:
            return ""
        sorted_params = {k: params[k] for k in sorted(params.keys())}
        return json.dumps(sorted_params, ensure_ascii=False, separators=(',', ':'))

    def _generate_signature(self, method, path, params, timestamp, nonce):
        """生成签名"""
        sorted_params = self._sort_params(params)
        
        # 构造签名内容: method + path + sorted_params + timestamp + nonce
        sign_content = f"{method}{path}{sorted_params}{timestamp}{nonce}"
        
        # HMAC-SHA256 并 hex 编码
        signature = hmac.new(
            self.app_secret.encode('utf-8'),
            sign_content.encode('utf-8'),
            hashlib.sha256
        ).hexdigest()
        
        return signature

    def _request(self, method, path, data=None):
        """发送请求"""
        timestamp = str(int(time.time()))
        nonce = str(uuid.uuid4())
        
        headers = {
            "Content-Type": "application/json",
            "X-App-Id": self.app_id,
            "X-Timestamp": timestamp,
            "X-Nonce": nonce,
            "X-Signature": self._generate_signature(method, path, data, timestamp, nonce)
        }
        
        url = f"{self.base_url}{path}"
        body = json.dumps(data, ensure_ascii=False) if data else None
        
        if method == "GET":
            response = requests.get(url, headers=headers, timeout=10)
        else:
            response = requests.post(url, headers=headers, data=body, timeout=10)
        
        return response.json()

    def send_message(self, channel_id, receiver, params=None, signature_name=None):
        """发送单条消息"""
        data = {
            "channel_id": channel_id,
            "receiver": receiver,
            "template_params": params or {}
        }
        if signature_name:
            data["signature_name"] = signature_name
        return self._request("POST", "/api/v1/messages", data)

    def send_batch(self, channel_id, receivers, params=None, signature_name=None):
        """批量发送消息"""
        data = {
            "channel_id": channel_id,
            "receivers": receivers,
            "template_params": params or {}
        }
        if signature_name:
            data["signature_name"] = signature_name
        return self._request("POST", "/api/v1/messages/batch", data)

    def query_task(self, task_id):
        """查询任务状态"""
        return self._request("GET", f"/api/v1/messages/{task_id}")


# 使用示例
if __name__ == "__main__":
    client = MessagePushClient(
        base_url="https://your-domain.com",
        app_id="your_app_id",
        app_secret="your_app_secret"
    )
    
    # 发送短信（通道已绑定消息类型和模板）
    result = client.send_message(
        channel_id=1,
        receiver="13800138000",
        params={"code": "123456"},
        signature_name="公司名称"  # 可选：指定签名名称
    )
    print(result)
    
    # 查询任务状态
    if result.get("code") == 0:
        task_id = result["data"]["task_id"]
        status = client.query_task(task_id)
        print(status)
```

---

### Go

```go
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// MessagePushClient 消息推送客户端
type MessagePushClient struct {
	BaseURL   string
	AppID     string
	AppSecret string
	Client    *http.Client
}

// NewMessagePushClient 创建客户端
func NewMessagePushClient(baseURL, appID, appSecret string) *MessagePushClient {
	return &MessagePushClient{
		BaseURL:   baseURL,
		AppID:     appID,
		AppSecret: appSecret,
		Client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// sortParams 按 key 排序参数
func (c *MessagePushClient) sortParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedMap := make(map[string]interface{})
	for _, k := range keys {
		sortedMap[k] = params[k]
	}
	result, _ := json.Marshal(sortedMap)
	return string(result)
}

// generateSignature 生成签名
func (c *MessagePushClient) generateSignature(method, path string, params map[string]interface{}, timestamp, nonce string) string {
	sortedParams := c.sortParams(params)
	
	// 构造签名内容: method + path + sorted_params + timestamp + nonce
	signContent := fmt.Sprintf("%s%s%s%s%s", method, path, sortedParams, timestamp, nonce)

	// HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(c.AppSecret))
	mac.Write([]byte(signContent))

	// Hex 编码
	return hex.EncodeToString(mac.Sum(nil))
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	ChannelID      int                    `json:"channel_id"`
	Receiver       string                 `json:"receiver"`
	TemplateParams map[string]interface{} `json:"template_params,omitempty"`
	SignatureName  string                 `json:"signature_name,omitempty"`
}

// Response 响应结构
type Response struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// SendMessage 发送单条消息
func (c *MessagePushClient) SendMessage(req *SendMessageRequest) (*Response, error) {
	path := "/api/v1/messages"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.New().String()
	
	// 构建参数 map 用于签名
	params := map[string]interface{}{
		"channel_id":      req.ChannelID,
		"receiver":        req.Receiver,
		"template_params": req.TemplateParams,
	}
	if req.SignatureName != "" {
		params["signature_name"] = req.SignatureName
	}
	
	signature := c.generateSignature("POST", path, params, timestamp, nonce)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-App-Id", c.AppID)
	httpReq.Header.Set("X-Timestamp", timestamp)
	httpReq.Header.Set("X-Nonce", nonce)
	httpReq.Header.Set("X-Signature", signature)

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// QueryTask 查询任务状态
func (c *MessagePushClient) QueryTask(taskID string) (*Response, error) {
	path := "/api/v1/messages/" + taskID
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.New().String()
	signature := c.generateSignature("GET", path, nil, timestamp, nonce)

	httpReq, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("X-App-Id", c.AppID)
	httpReq.Header.Set("X-Timestamp", timestamp)
	httpReq.Header.Set("X-Nonce", nonce)
	httpReq.Header.Set("X-Signature", signature)

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result Response
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func main() {
	client := NewMessagePushClient(
		"https://your-domain.com",
		"your_app_id",
		"your_app_secret",
	)

	// 发送短信（通道已绑定消息类型和模板）
	resp, err := client.SendMessage(&SendMessageRequest{
		ChannelID: 1,
		Receiver:  "13800138000",
		TemplateParams: map[string]interface{}{
			"code": "123456",
		},
		SignatureName: "公司名称", // 可选：指定签名名称
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %+v\n", resp)
}
```

---

### Java

```java
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.HashMap;
import java.util.Map;
import java.util.TreeMap;
import java.util.UUID;

import com.fasterxml.jackson.databind.ObjectMapper;

public class MessagePushClient {
    private final String baseUrl;
    private final String appId;
    private final String appSecret;
    private final HttpClient httpClient;
    private final ObjectMapper objectMapper;

    public MessagePushClient(String baseUrl, String appId, String appSecret) {
        this.baseUrl = baseUrl;
        this.appId = appId;
        this.appSecret = appSecret;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .build();
        this.objectMapper = new ObjectMapper();
    }

    /**
     * 按 key 排序参数
     */
    private String sortParams(Map<String, Object> params) throws Exception {
        if (params == null || params.isEmpty()) {
            return "";
        }
        TreeMap<String, Object> sortedMap = new TreeMap<>(params);
        return objectMapper.writeValueAsString(sortedMap);
    }

    /**
     * 生成签名
     */
    private String generateSignature(String method, String path, 
            Map<String, Object> params, String timestamp, String nonce) throws Exception {
        String sortedParams = sortParams(params);

        // 构造签名内容: method + path + sorted_params + timestamp + nonce
        String signContent = method + path + sortedParams + timestamp + nonce;

        // HMAC-SHA256
        Mac mac = Mac.getInstance("HmacSHA256");
        SecretKeySpec secretKeySpec = new SecretKeySpec(
                appSecret.getBytes(StandardCharsets.UTF_8), "HmacSHA256");
        mac.init(secretKeySpec);
        byte[] hmacBytes = mac.doFinal(signContent.getBytes(StandardCharsets.UTF_8));

        // Hex 编码
        return bytesToHex(hmacBytes);
    }

    private String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    /**
     * 发送单条消息
     */
    public Map<String, Object> sendMessage(int channelId, String receiver, 
            Map<String, Object> params, String signatureName) throws Exception {
        
        String path = "/api/v1/messages";
        String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
        String nonce = UUID.randomUUID().toString();
        
        Map<String, Object> requestBody = new HashMap<>();
        requestBody.put("channel_id", channelId);
        requestBody.put("receiver", receiver);
        requestBody.put("template_params", params != null ? params : new HashMap<>());
        if (signatureName != null && !signatureName.isEmpty()) {
            requestBody.put("signature_name", signatureName);
        }

        String signature = generateSignature("POST", path, requestBody, timestamp, nonce);
        String body = objectMapper.writeValueAsString(requestBody);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + path))
                .header("Content-Type", "application/json")
                .header("X-App-Id", appId)
                .header("X-Timestamp", timestamp)
                .header("X-Nonce", nonce)
                .header("X-Signature", signature)
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .build();

        HttpResponse<String> response = httpClient.send(request, 
                HttpResponse.BodyHandlers.ofString());

        return objectMapper.readValue(response.body(), Map.class);
    }

    /**
     * 查询任务状态
     */
    public Map<String, Object> queryTask(String taskId) throws Exception {
        String path = "/api/v1/messages/" + taskId;
        String timestamp = String.valueOf(System.currentTimeMillis() / 1000);
        String nonce = UUID.randomUUID().toString();
        String signature = generateSignature("GET", path, null, timestamp, nonce);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + path))
                .header("X-App-Id", appId)
                .header("X-Timestamp", timestamp)
                .header("X-Nonce", nonce)
                .header("X-Signature", signature)
                .GET()
                .build();

        HttpResponse<String> response = httpClient.send(request, 
                HttpResponse.BodyHandlers.ofString());

        return objectMapper.readValue(response.body(), Map.class);
    }

    public static void main(String[] args) throws Exception {
        MessagePushClient client = new MessagePushClient(
                "https://your-domain.com",
                "your_app_id",
                "your_app_secret"
        );

        // 发送短信（通道已绑定消息类型和模板）
        Map<String, Object> params = new HashMap<>();
        params.put("code", "123456");

        // 可选：指定签名名称
        Map<String, Object> result = client.sendMessage(1, "13800138000", params, "公司名称");
        System.out.println("Result: " + result);

        // 查询任务状态
        if ((int) result.get("code") == 0) {
            Map<String, Object> data = (Map<String, Object>) result.get("data");
            String taskId = (String) data.get("task_id");
            Map<String, Object> status = client.queryTask(taskId);
            System.out.println("Status: " + status);
        }
    }
}
```

---

### Node.js

```javascript
const crypto = require('crypto');
const https = require('https');
const http = require('http');

class MessagePushClient {
    constructor(baseUrl, appId, appSecret) {
        this.baseUrl = baseUrl;
        this.appId = appId;
        this.appSecret = appSecret;
    }

    /**
     * 按 key 排序参数
     */
    sortParams(params) {
        if (!params || Object.keys(params).length === 0) {
            return '';
        }
        const sortedKeys = Object.keys(params).sort();
        const sortedObj = {};
        for (const key of sortedKeys) {
            sortedObj[key] = params[key];
        }
        return JSON.stringify(sortedObj);
    }

    /**
     * 生成签名
     */
    generateSignature(method, path, params, timestamp, nonce) {
        const sortedParams = this.sortParams(params);

        // 构造签名内容: method + path + sorted_params + timestamp + nonce
        const signContent = `${method}${path}${sortedParams}${timestamp}${nonce}`;

        // HMAC-SHA256 并 hex 编码
        const signature = crypto
            .createHmac('sha256', this.appSecret)
            .update(signContent, 'utf8')
            .digest('hex');

        return signature;
    }

    /**
     * 发送 HTTP 请求
     */
    async request(method, path, data = null) {
        const timestamp = Math.floor(Date.now() / 1000).toString();
        const nonce = crypto.randomUUID();
        const signature = this.generateSignature(method, path, data, timestamp, nonce);

        const url = new URL(path, this.baseUrl);
        const isHttps = url.protocol === 'https:';
        const httpModule = isHttps ? https : http;

        const options = {
            hostname: url.hostname,
            port: url.port || (isHttps ? 443 : 80),
            path: url.pathname,
            method: method,
            headers: {
                'Content-Type': 'application/json',
                'X-App-Id': this.appId,
                'X-Timestamp': timestamp,
                'X-Nonce': nonce,
                'X-Signature': signature,
            },
        };

        const body = data ? JSON.stringify(data) : '';

        return new Promise((resolve, reject) => {
            const req = httpModule.request(options, (res) => {
                let responseData = '';
                res.on('data', (chunk) => {
                    responseData += chunk;
                });
                res.on('end', () => {
                    try {
                        resolve(JSON.parse(responseData));
                    } catch (e) {
                        reject(new Error('Invalid JSON response'));
                    }
                });
            });

            req.on('error', reject);
            req.setTimeout(10000, () => {
                req.destroy();
                reject(new Error('Request timeout'));
            });

            if (body) {
                req.write(body);
            }
            req.end();
        });
    }

    /**
     * 发送单条消息
     */
    async sendMessage(channelId, receiver, params = {}, signatureName = null) {
        const data = {
            channel_id: channelId,
            receiver: receiver,
            template_params: params,
        };
        if (signatureName) {
            data.signature_name = signatureName;
        }
        return this.request('POST', '/api/v1/messages', data);
    }

    /**
     * 批量发送消息
     */
    async sendBatch(channelId, receivers, params = {}, signatureName = null) {
        const data = {
            channel_id: channelId,
            receivers: receivers,
            template_params: params,
        };
        if (signatureName) {
            data.signature_name = signatureName;
        }
        return this.request('POST', '/api/v1/messages/batch', data);
    }

    /**
     * 查询任务状态
     */
    async queryTask(taskId) {
        return this.request('GET', `/api/v1/messages/${taskId}`);
    }
}

// 使用示例
async function main() {
    const client = new MessagePushClient(
        'https://your-domain.com',
        'your_app_id',
        'your_app_secret'
    );

    try {
        // 发送短信（通道已绑定消息类型和模板）
        const result = await client.sendMessage(
            1,
            '13800138000',
            { code: '123456' },
            '公司名称'  // 可选：指定签名名称
        );
        console.log('Send result:', result);

        // 查询任务状态
        if (result.code === 0) {
            const taskId = result.data.task_id;
            const status = await client.queryTask(taskId);
            console.log('Task status:', status);
        }
    } catch (error) {
        console.error('Error:', error.message);
    }
}

main();
```

---

### PHP

```php
<?php

class MessagePushClient
{
    private string $baseUrl;
    private string $appId;
    private string $appSecret;

    public function __construct(string $baseUrl, string $appId, string $appSecret)
    {
        $this->baseUrl = rtrim($baseUrl, '/');
        $this->appId = $appId;
        $this->appSecret = $appSecret;
    }

    /**
     * 按 key 排序参数
     */
    private function sortParams(?array $params): string
    {
        if ($params === null || empty($params)) {
            return '';
        }
        ksort($params);
        return json_encode($params, JSON_UNESCAPED_UNICODE);
    }

    /**
     * 生成签名
     */
    private function generateSignature(
        string $method, 
        string $path, 
        ?array $params, 
        string $timestamp, 
        string $nonce
    ): string {
        $sortedParams = $this->sortParams($params);

        // 构造签名内容: method + path + sorted_params + timestamp + nonce
        $signContent = $method . $path . $sortedParams . $timestamp . $nonce;

        // HMAC-SHA256 并 hex 编码
        $signature = hash_hmac('sha256', $signContent, $this->appSecret);

        return $signature;
    }

    /**
     * 发送 HTTP 请求
     */
    private function request(string $method, string $path, ?array $data = null): array
    {
        $timestamp = (string) time();
        $nonce = bin2hex(random_bytes(16));
        $signature = $this->generateSignature($method, $path, $data, $timestamp, $nonce);

        $url = $this->baseUrl . $path;
        $body = $data ? json_encode($data, JSON_UNESCAPED_UNICODE) : '';

        $headers = [
            'Content-Type: application/json',
            'X-App-Id: ' . $this->appId,
            'X-Timestamp: ' . $timestamp,
            'X-Nonce: ' . $nonce,
            'X-Signature: ' . $signature,
        ];

        $ch = curl_init();
        curl_setopt($ch, CURLOPT_URL, $url);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_TIMEOUT, 10);
        curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);

        if ($method === 'POST') {
            curl_setopt($ch, CURLOPT_POST, true);
            curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
        }

        $response = curl_exec($ch);
        $error = curl_error($ch);
        curl_close($ch);

        if ($error) {
            throw new Exception('CURL Error: ' . $error);
        }

        return json_decode($response, true);
    }

    /**
     * 发送单条消息
     */
    public function sendMessage(int $channelId, string $receiver, array $params = [], ?string $signatureName = null): array
    {
        $data = [
            'channel_id' => $channelId,
            'receiver' => $receiver,
            'template_params' => $params,
        ];
        if ($signatureName !== null) {
            $data['signature_name'] = $signatureName;
        }
        return $this->request('POST', '/api/v1/messages', $data);
    }

    /**
     * 批量发送消息
     */
    public function sendBatch(int $channelId, array $receivers, array $params = [], ?string $signatureName = null): array
    {
        $data = [
            'channel_id' => $channelId,
            'receivers' => $receivers,
            'template_params' => $params,
        ];
        if ($signatureName !== null) {
            $data['signature_name'] = $signatureName;
        }
        return $this->request('POST', '/api/v1/messages/batch', $data);
    }

    /**
     * 查询任务状态
     */
    public function queryTask(string $taskId): array
    {
        return $this->request('GET', '/api/v1/messages/' . $taskId);
    }
}

// 使用示例
$client = new MessagePushClient(
    'https://your-domain.com',
    'your_app_id',
    'your_app_secret'
);

try {
    // 发送短信（通道已绑定消息类型和模板）
    $result = $client->sendMessage(
        1,
        '13800138000',
        ['code' => '123456'],
        '公司名称'  // 可选：指定签名名称
    );
    print_r($result);

    // 查询任务状态
    if ($result['code'] === 0) {
        $taskId = $result['data']['task_id'];
        $status = $client->queryTask($taskId);
        print_r($status);
    }
} catch (Exception $e) {
    echo 'Error: ' . $e->getMessage() . PHP_EOL;
}
```

---

## 错误码参考

### 请求错误 (1xxxx)

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 10001 | 请求参数错误 | 检查请求参数格式 |
| 10002 | JSON 格式错误 | 检查 JSON 语法 |
| 10003 | 缺少必填参数 | 补充必填字段 |
| 10004 | 参数值非法 | 检查参数取值范围 |
| 10005 | 接收者格式错误 | 检查手机号/邮箱格式 |
| 10006 | 模板参数错误 | 检查模板参数是否匹配 |

### 鉴权错误 (2xxxx)

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 20001 | 未授权 | 检查认证头是否完整 |
| 20002 | 无效的 AppID | 检查 app_id 是否正确 |
| 20003 | 签名验证失败 | 检查签名算法和密钥 |
| 20004 | 时间戳无效 | 检查时间戳是否在 ±5 分钟内 |
| 20005 | IP 不在白名单 | 联系管理员添加 IP |
| 20006 | 应用已禁用 | 联系管理员启用应用 |

### 业务错误 (3xxxx)

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 30001 | 超出速率限制 | 降低请求频率 |
| 30002 | 超出配额限制 | 联系管理员提升配额 |
| 30003 | 通道不存在 | 检查 channel_id |
| 30004 | 通道已禁用 | 使用其他通道 |
| 30005 | 模板不存在 | 检查 template_id |
| 30006 | 无可用通道 | 联系管理员配置通道 |
| 30007 | 任务不存在 | 检查 task_id |
| 30008 | 批量任务不存在 | 检查 batch_id |

### 系统错误 (4xxxx)

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| 40001 | 内部错误 | 稍后重试或联系技术支持 |
| 40002 | 数据库错误 | 联系技术支持 |
| 40003 | Redis 错误 | 联系技术支持 |
| 40004 | 队列错误 | 联系技术支持 |
| 40005 | 服务商错误 | 检查服务商配置 |
| 40006 | 网络超时 | 稍后重试 |
| 40007 | 熔断器打开 | 等待服务恢复 |

---

## 附录

### A. 消息类型

| 类型 | 值 | 说明 |
|------|------|------|
| 短信 | `sms` | 手机短信 |
| 邮件 | `email` | 电子邮件 |
| 企业微信 | `wechat_work` | 企业微信应用消息 |
| 钉钉 | `dingtalk` | 钉钉工作通知 |
| Webhook | `webhook` | HTTP 回调 |
| 推送通知 | `push` | APP 推送通知 |

### B. 任务状态

| 状态 | 值 | 说明 |
|------|------|------|
| 待处理 | `pending` | 任务已创建，等待发送 |
| 处理中 | `processing` | 任务正在发送 |
| 已发送 | `sent` | 已发送，等待回调确认（短信等类型） |
| 成功 | `success` | 发送成功 |
| 失败 | `failed` | 发送失败（已达最大重试次数） |

### C. 回调状态

| 状态 | 值 | 说明 |
|------|------|------|
| 已送达 | `delivered` | 消息已送达接收者 |
| 发送失败 | `failed` | 运营商/服务商发送失败 |
| 被拒绝 | `rejected` | 接收者拒收 |
| 超时 | `timeout` | 等待回调超时 |

### D. 最佳实践

1. **Nonce 唯一性**：每次请求使用不同的 nonce，防止重放攻击
2. **重试策略**：服务端已内置重试（最多 3 次），客户端无需重复提交
3. **批量限制**：单次批量发送建议不超过 500 条
4. **超时设置**：建议请求超时设置为 10 秒
5. **日志记录**：保存 task_id 便于问题排查
6. **通道配置**：消息类型和模板已绑定到通道，调用时只需传 channel_id

---

**文档版本**: v1.3.0  
**更新日期**: 2025-11-27

