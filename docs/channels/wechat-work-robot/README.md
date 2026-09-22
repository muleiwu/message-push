# 企业微信群机器人（wechat_work_robot）

## 概述

企业微信群机器人（Webhook 群机器人）渠道，消息类型为 `wechat_work`，provider code 为 `wechat_work_robot`（`constants.ProviderWeChatWorkRobot`）。实现位于 `modules/sender/infrastructure/`。只实现基础 `Sender`——无批量、无回调、无状态查询，是能力最精简的渠道之一。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ❌ | `SupportsBatchSend=false` |
| 回调 `CallbackHandler` | ❌ | `SupportsCallback=false` |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 无 | — |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `webhook_url` | url | 是 | 群机器人 Webhook 地址（含 `key` 查询参数） |
| `msg_type` | select | 是 | `text`（默认）/ `markdown`，**账号级配置**（非模板 ContentType） |

## 模板与变量

- **系统模板**：直接使用渲染结果，无 `template_code`。
- **消息类型**：由账号配置 `msg_type` 决定——同一账号发出的所有消息类型一致；需要 text/markdown 混用请建多个账号。

## 发送与回执

- **@ 列表**：接收者字段为逗号分隔的 @ 列表，规则：
  - 纯数字 → `mentioned_mobile_list`（手机号）；
  - 其余 → `mentioned_list`（userid）；
  - `@all` / `all` → @ 全体。
- **markdown 特例**：企业微信 markdown 消息**不支持** mentioned_list，需在内容里用 `<@userid>` 前缀行；纯手机号条目会被跳过。
- **回执**：无回调。发送接口同步返回即结果，成功即终态。

## 限制与注意事项

- **频率限制**：企业微信侧每个机器人约 20 条/分钟，**代码未实现客户端限流**——高频发送会被上游拒绝，需靠应用层限流或规则引擎重试兜底。
- 无批量能力：多接收者任务会逐条发送。
- Webhook URL 等同于凭证，泄露即可向群内发消息，注意账号配置的访问控制。

## 相关文件

- `modules/sender/infrastructure/wechat_work_robot_sender.go` — 发送与 @ 解析
- `modules/sender/infrastructure/robot_sender_test.go` — 测试

## 相关文档

- [渠道总览](../README.md)
