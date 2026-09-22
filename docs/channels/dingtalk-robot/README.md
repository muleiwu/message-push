# 钉钉群机器人（dingtalk_robot）

## 概述

钉钉群机器人（自定义 Webhook 机器人）渠道，消息类型为 `dingtalk`，provider code 为 `dingtalk_robot`（`constants.ProviderDingTalkRobot`）。实现位于 `modules/sender/infrastructure/`。只实现基础 `Sender`，支持可选的加签（SEC 密钥）防伪造。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ❌ | — |
| 回调 `CallbackHandler` | ❌ | — |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 无 | markdown 标题取 `Signature`，默认"通知" |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `webhook_url` | url | 是 | 群机器人 Webhook 地址（含 `access_token` 参数） |
| `secret` | password | 否 | `SEC` 开头的加签密钥，敏感字段 |
| `msg_type` | select | 是 | `text`（默认）/ `markdown`，账号级配置 |

## 模板与变量

- **系统模板**：直接使用渲染结果，无 `template_code`。
- **消息类型**：由账号配置 `msg_type` 决定；markdown 标题取任务的 `Signature` 字段，缺省为"通知"。

## 发送与回执

- **加签**：配置 `secret` 后，每个请求追加 `timestamp&sign` 查询参数，`sign = base64(HMAC-SHA256(secret, timestamp + "\n" + secret))`；未配置则不加签。
- **@ 列表**：接收者字段为逗号分隔列表：
  - 纯数字 → `atMobiles`（手机号）；
  - 其余 → `atUserIds`（userid）；
  - `@all` → `isAtAll` 全体。
- **回执**：无回调。发送接口同步返回即结果，成功即终态。

## 限制与注意事项

- **频率限制**：钉钉侧每个机器人约 20 条/分钟，**代码未实现客户端限流**——高频发送会被上游拒绝（错误码限流），需靠应用层限流或规则引擎重试兜底。
- 无批量能力：多接收者任务会逐条发送。
- Webhook URL + secret 等同于凭证，注意账号配置的访问控制。

## 相关文件

- `modules/sender/infrastructure/dingtalk_robot_sender.go` — 发送/加签/@ 解析
- `modules/sender/infrastructure/robot_sender_test.go` — 测试

## 相关文档

- [渠道总览](../README.md)
