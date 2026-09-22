# 企业微信应用消息（wechat_work）

## 概述

企业微信自建应用消息渠道，消息类型为 `wechat_work`，provider code 为 `wechat_work`（`constants.ProviderWeChatWork`）。实现位于 `modules/sender/infrastructure/`。使用**系统模板**渲染，无供应商签名概念。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | userid 用 `\|` 拼接一次请求 |
| 回调 `CallbackHandler` | ✅ | 不验签 |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 无 | — |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `corp_id` | text | 是 | 企业 ID |
| `agent_secret` | password | 是 | 应用 Secret，敏感字段 |
| `agent_id` | text | 是 | 应用 ID，必须可解析为整数 |

## 模板与变量

- **系统模板**：直接使用渲染结果，无 `template_code`。
- **消息类型**：`msgtype` 为 `text` 或 `markdown`，由供应商模板的 `ContentType` 决定（`content_type.go`）。
- **签名**：无签名概念。

## 发送与回执

- **接收者**：`touser` 为成员 userid，**为空时发送给全员（@all）**。
- **批量**：多个 userid 用 `|` 连接为一次请求；结果区分 `invalid_user`（无效用户）与 `unlicensed_user`（无授权用户）分别记为失败。
- **access_token**：缓存于 Redis（有效期为上游 `expires_in` 减 200 秒），避免频繁获取。
- **回调**：`POST/GET /api/callback/:id`；请求体为 JSON（`msgid`/`status:SEND_OK`），不验签，响应 `{"errcode":0,"errmsg":"ok"}`。
- **终态**：发送成功即视为终态。

## 限制与注意事项

- **content 上限 2048 字节**（UTF-8 安全截断，不会截断多字节字符——但内容超长仍会丢内容，模板侧应控制长度）。
- markdown 消息同样受 2048 字节限制。
- 回调不验签，回调地址应视为半公开。

## 相关文件

- `modules/sender/infrastructure/wechat_work_sender.go` — 发送/批量/回调/token 缓存
- `modules/sender/infrastructure/content_type.go` — ContentType → msgtype 映射
- `modules/sender/infrastructure/robot_sender_test.go` — 机器人相关测试（共用逻辑）

## 相关文档

- [渠道总览](../README.md)
