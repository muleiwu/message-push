# 钉钉工作通知（dingtalk）

## 概述

钉钉企业内部应用工作通知渠道，消息类型为 `dingtalk`，provider code 为 `dingtalk`（`constants.ProviderDingTalk`）。实现位于 `modules/sender/infrastructure/`。使用**系统模板**渲染；是全部渠道中**唯一实现回调验签与加解密**的供应商。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | userid 逗号拼接一次 `asyncsend_v2` |
| 回调 `CallbackHandler` | ✅ | SHA1 验签 + AES-CBC 解密（加密模式） |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 无 | markdown 标题取 `task.Signature`，默认"消息" |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `app_key` | text | 是 | 应用的 AppKey |
| `app_secret` | password | 是 | 敏感字段 |
| `agent_id` | text | 是 | 应用 AgentId，工作通知必填 |
| `callback_token` | text | 否 | 回调验签 Token |
| `callback_aes_key` | password | 否 | 回调 AES 密钥，43 位，敏感字段 |

## 模板与变量

- **系统模板**：直接使用渲染结果，无 `template_code`。
- **消息类型**：`text` / `markdown` 由供应商模板的 `ContentType` 决定（`content_type.go`）；markdown 消息的标题取任务的 `Signature` 字段，缺省为"消息"。
- **签名**：无签名概念（`Signature` 复用为 markdown 标题）。

## 发送与回执

- **批量**：多个 userid 逗号拼接为一次 `asyncsend_v2` 请求，全批共用一个 `task_id` 关联回执。
- **access_token**：缓存于 Redis（key `dingtalk:token:{appKey}`）。
- **回调（加密模式）**：`POST/GET /api/callback/:id`：
  - 验签：`signature = SHA1(字典序排序(token, timestamp, nonce, encrypt))`；
  - 解密：AES-CBC，key = Base64(EncodingAESKey + `"="`)，IV = key 前 16 字节，明文格式为 `random(16) + len(4) + msg + corpid`；
  - token / aesKey / corpID 由上层经 Header `X-Callback-Token` / `X-Callback-AesKey` / `X-Callback-CorpId` 注入；未配置 aesKey 时回退明文解析；
  - `check_url` 事件直接忽略（钉钉探活）；响应为加密的 `"success"`。

## 限制与注意事项

- 工作通知有每日数量上限（钉钉侧按企业规模限制），大量发送需评估。
- 回调要正常验签解密，账号必须配置 `callback_token` 与 `callback_aes_key`，且与钉钉开放平台的加密配置一致。
- 发送成功即终态，无状态补齐手段。

## 相关文件

- `modules/sender/infrastructure/dingtalk_sender.go` — 发送/批量/回调/验签解密/token 缓存
- `modules/sender/infrastructure/content_type.go` — ContentType → msgtype 映射

## 相关文档

- [渠道总览](../README.md)
