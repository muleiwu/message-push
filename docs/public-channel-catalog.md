# 开放通道目录与可视化接入

业务系统可以通过开放 API 读取通道、绑定的系统模板和可发送签名别名，构建“选择通道 → 查看模板 → 填写变量 → 选择签名 → 发送”的联动表单。一个通道绑定一个系统模板，发送时继续使用 `channel_id`，无需传递模板 ID。

所有有效应用共享已启用的通道。目录查询使用应用 HMAC 鉴权和现有限流，不检查或消耗每日发送配额；发送额度耗尽时仍可读取配置。由业务后端保管应用密钥、签名并转发查询，前端只接收表单配置。

## 1. 获取通道列表

```http
GET /api/v1/channels?type=sms&page=1&page_size=20
X-App-Id: your-app-id
X-Timestamp: <Unix 秒级时间戳>
X-Nonce: <随机字符串>
X-Signature: <HMAC-SHA256 十六进制签名>
```

| 参数 | 规则 |
| --- | --- |
| `type` | 可选：`sms`、`email`、`wechat_work`、`dingtalk`、`webhook`、`push`；省略时返回全部类型 |
| `page` | 可选，默认 `1`，必须大于等于 `1` |
| `page_size` | 可选，默认 `20`，范围 `1–100` |

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": 42,
        "name": "登录验证码通道",
        "type": "sms",
        "message_template_id": 7,
        "template_name": "登录验证码",
        "readiness": { "state": "ready", "blocker_codes": [] }
      }
    ],
    "total": 1,
    "page": 1,
    "size": 20
  }
}
```

列表按通道 ID 倒序，空页的 `items` 为 `[]`，`total` 为筛选后的总数。列表保留已启用但配置不完整的通道；禁用、删除的通道不可见。

## 2. 获取模板、变量和签名选项

```http
GET /api/v1/channels/42
```

同样携带四个认证头；详情签名必须使用详情路径，不能复用列表请求的签名。

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 42,
    "name": "登录验证码通道",
    "type": "sms",
    "message_template_id": 7,
    "template_name": "登录验证码",
    "readiness": { "state": "ready", "blocker_codes": [] },
    "template": {
      "id": 7,
      "template_name": "登录验证码",
      "content_type": "text",
      "content": "您的验证码是{code}，{expire}分钟内有效。",
      "variables": ["code", "expire"],
      "description": "用于登录验证"
    },
    "signature_required": true,
    "signature_names": ["验证码"]
  }
}
```

- `template.content` 是系统模板原文，可配合 `content_type` 展示；通常为 `text/html/markdown`，内容类型未配置时返回 `text`。供应商实际投递内容仍由通道的供应商模板和参数映射决定。
- `template.variables` 保留配置顺序，每个变量对应一个字符串输入值，所有变量键都要传入 `template_params`。有效无变量时返回 `[]`。
- `signature_names` 是去重并排序后的公共别名，直接作为选择框的展示文字和提交值。它适用于短信签名、邮件标题以及 OneBot 的可选签名。实际签名值、供应商账号及映射明细不在公开响应中。
- `signature_required` 与发送校验一致；为 `true` 时必须选择返回的别名。OneBot 返回 `false`，签名选项可以非空，选择框应可清空、默认不选。通道不可发送时返回 `false` 和空选项。
- 多供应商通道只提供有效发送路径中签名账号共同支持的别名，避免选择后因供应商切换而找不到映射。OneBot 未配置共同别名时仍允许无签名发送；传入非空别名则必须有有效映射。

### 状态和异常配置

| 情况 | 返回及业务系统行为 |
| --- | --- |
| `readiness.state = ready` | 配置就绪，可选 |
| `readiness.state = degraded` | 部分路径异常，仍有可发送路径，可选并展示提示 |
| `readiness.state = blocked` | 当前不能发送，置灰并展示原因；签名选项为 `[]` |
| 系统模板缺失或被删除 | `template: null`、`template_name: ""`，状态为 `blocked` |
| 系统模板被禁用 | 保留模板内容，状态为 `blocked`，原因码包含 `MESSAGE_TEMPLATE_DISABLED` |
| 变量 JSON 损坏、名称空白或重复 | `variables: null`，状态为 `blocked`，原因码包含 `MESSAGE_TEMPLATE_VARIABLES_INVALID` |

`blocker_codes` 使用现有稳定原因码，例如 `MESSAGE_TEMPLATE_MISSING`、`NO_ACTIVE_BINDING`、`PROVIDER_ACCOUNT_UNAVAILABLE`、`PROVIDER_TEMPLATE_UNAVAILABLE`、`PARAM_MAPPING_INVALID`、`SIGNATURE_REQUIRED`、`SIGNATURE_ALIAS_NOT_SHARED`。响应只包含原因码，不包含供应商账号或绑定 ID。

查询直接读取当前配置。就绪状态描述查询时的配置，不预留发送额度或供应商容量；真正发送时仍会检查配额和当前配置。

## 3. 构建发送请求

选取通道 `42`、签名别名 `验证码`，根据返回的变量名收集表单值：

```http
POST /api/v1/messages
Content-Type: application/json
```

```json
{
  "channel_id": 42,
  "receiver": "13800138000",
  "signature_name": "验证码",
  "template_params": { "code": "123456", "expire": "5" }
}
```

批量发送继续使用 `/api/v1/messages/batch`，将 `receiver` 换成 `receivers` 数组。无需签名时可省略 `signature_name`；无变量时可传 `template_params: {}`。系统模板 ID、供应商模板代码均无需出现在发送请求中。

## GET 签名示例

沿用现有签名算法：`HMAC-SHA256(app_secret, method + path + sorted_body_params + timestamp + nonce)`。GET 不带正文时 `sorted_body_params` 是空字符串，`path` 包含 `/api/v1` 且不含查询串。查询参数不参与当前签名。

下面的 Python 示例只查询配置，使用业务后端环境变量 `PUSH_BASE_URL`、`PUSH_APP_ID`、`PUSH_APP_SECRET`，可选 `PUSH_CHANNEL_ID` 指定详情：

```python
import hashlib
import hmac
import json
import os
import time
import uuid
from urllib.parse import urlsplit
from urllib.request import Request, urlopen


def get_catalog(path_with_query):
    url = os.environ["PUSH_BASE_URL"].rstrip("/") + path_with_query
    timestamp = str(int(time.time()))
    nonce = str(uuid.uuid4())
    content = "GET" + urlsplit(url).path + timestamp + nonce
    signature = hmac.new(
        os.environ["PUSH_APP_SECRET"].encode("utf-8"),
        content.encode("utf-8"),
        hashlib.sha256,
    ).hexdigest()
    request = Request(url, headers={
        "X-App-Id": os.environ["PUSH_APP_ID"],
        "X-Timestamp": timestamp,
        "X-Nonce": nonce,
        "X-Signature": signature,
    })
    with urlopen(request, timeout=10) as response:
        result = json.load(response)
    if result["code"] != 0:
        raise RuntimeError(f"{result['code']}: {result['message']}")
    return result["data"]


page = get_catalog("/api/v1/channels?type=sms&page=1&page_size=20")
print(json.dumps(page, ensure_ascii=False, indent=2))
if os.environ.get("PUSH_CHANNEL_ID"):
    channel_id = int(os.environ["PUSH_CHANNEL_ID"])
    detail = get_catalog(f"/api/v1/channels/{channel_id}")
    print(json.dumps(detail, ensure_ascii=False, indent=2))
```

POST 发送请求仍需对 JSON 正文参与签名。可复用现有 [Apifox 签名脚本](../scripts/apifox-signature.js) 调试上述查询和发送请求；在 GET 请求中清空正文。

## 错误处理

非法 ID、分页或类型返回 HTTP 400、`code: 400`；通道不存在、已删除或已禁用返回 HTTP 404、`code: 404`；数据库或查询故障返回 HTTP 500、`code: 500`。

鉴权与限流保持现有协议：错误可能使用 HTTP 200，应用鉴权相关业务码为 `20001–20006`，限流为 `30001`。接入方应同时检查 HTTP 状态与 `code`，仅将 `code: 0` 视为成功。

完整接口结构见 [Swagger](swagger.json)。
