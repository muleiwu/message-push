# Webhook 通知

应用配置 Webhook URL 后，系统会为任务终态和上行短信创建持久化待投递记录。任务重试或切换供应商期间不通知，只有最终状态生成事件。

## 事件

| event | 含义 |
|---|---|
| `success` | 无需供应商回执的消息同步完成 |
| `failed` | 发送、入队或处理最终失败 |
| `delivered` | 供应商回执确认送达 |
| `rejected` | 供应商回执确认拒绝 |
| `upstream` | 收到用户上行短信 |

## 配置优先级

存在独立的 Webhook 配置时，以该配置的启用状态、事件列表、签名密钥、重试次数和超时时间为准；配置被禁用或未订阅某事件时，不会绕过它使用应用 URL。

没有独立配置时，系统使用应用的 Webhook URL，默认订阅以上全部事件，最多重试 3 次、单次超时 5 秒且不签名。

## 请求与幂等

Webhook 使用 `POST application/json`。请求头包含：

- `X-Webhook-Event`：事件类型。
- `X-Webhook-Delivery-ID`：稳定的投递编号；同一事件重试时保持不变，接收方应据此去重。
- `X-Webhook-Timestamp`：本次 HTTP 尝试的 Unix 时间戳。
- `X-Webhook-Signature`：配置签名密钥时提供，值为 `HMAC-SHA256(timestamp + "." + body)` 的十六进制结果。

payload 中的 `timestamp` 是事件发生时间，不随重试改变。系统对非 `2xx` 响应或网络错误进行持久化重试，因此投递语义为 at-least-once。

管理端投递日志状态为 `pending`（待投递）、`processing`（投递中）、`success`（成功）或 `failed`（重试耗尽）。
