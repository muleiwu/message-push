# QQ（OneBot 11）（onebot）

## 概述

QQ 渠道，通过 OneBot 11 协议（兼容 go-cqhttp / NapCat / Lagrange 等实现）对接，消息类型为 `qq`，provider code 为 `onebot`（`constants.ProviderOneBot`）。实现位于 `modules/sender/infrastructure/`。模板**本地渲染**——`template_code` 只是本地唯一名，无需在上游注册。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ❌ | — |
| 回调 `CallbackHandler` | ❌ | — |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 可选 | `SupportsSignature=true`，自动补 `【】` |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `base_url` | url | 是 | OneBot 实现 的 HTTP 端点；http/https、可带路径前缀、禁 userinfo/query/fragment |
| `access_token` | password | 否 | Bearer 头，敏感字段（诊断日志脱敏） |
| `message_format` | select | 是 | `text`（默认）/ `cqcode` |

## 模板与变量

- **本地模板**：`template_code` 仅作为本地模板唯一名，上游无需注册。
- **签名**：可选。配置后作为内容前缀，自动补 `【】` 括号。
- **cqcode 转义**：`message_format=cqcode` 时，签名中的 `&`、`[`、`]` 会被转义，避免破坏 CQ 码。

## 发送与回执

- **接收者格式**：`private:123456`（私聊 QQ 号）/ `group:987654`（群号），由 `helper.ParseQQReceiver` 解析。
- **异步受理拒绝**：上游返回 `status=async` 或 `retcode=1`（异步受理）时，报错 `ErrorCodeOneBotAsyncUnsupported`，**默认不重试**，防止上游异步补发导致重复消息。
- **回执**：无回调。发送接口同步返回即结果。

## 限制与注意事项

- **安全限制**（代码内置）：禁止跟随重定向；响应体上限 1 MiB；请求超时 10 秒。
- OneBot 实现本身不走官方服务器，账号风控/冻结风险由所用实现与 QQ 账号决定，生产使用需评估合规性。
- 异步模式不重试意味着这类失败只能人工处理或换通道，接入前确认 OneBot 实现支持同步返回。

## 相关文件

- `modules/sender/infrastructure/onebot_sender.go` — 发送/接收者解析/安全限制
- `modules/sender/infrastructure/onebot_sender_test.go` / `onebot_signature_test.go` — 测试
- `app/readiness/onebot_signature_test.go` — 发送就绪检查测试

## 相关文档

- [QQ（OneBot 11）接入指南](../../onebot.md) — 配置、接收者格式、发送 API、异步限制
- [渠道总览](../README.md)
