# 网易云信短信（netease_sms）

## 概述

网易云信短信渠道，消息类型为 `sms`，provider code 为 `netease_sms`（`constants.ProviderNeteaseSMS`）。实现位于 `modules/sender/infrastructure/`。与阿里云/腾讯云不同：不做资源管理（模板在网易云信控制台维护），也没有状态查询/拉取——回执完全依赖回调。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | 单请求 ≤100 手机号，超出自动分批 |
| 回调 `CallbackHandler` | ✅ | 不校验 CheckSum |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | 模板/签名在网易云信控制台维护 |
| 供应商签名 | 系统侧必须 | 仅做配置校验，不传给 API（签名在网易云信与模板绑定） |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `app_key` | text | 是 | — |
| `app_secret` | password | 是 | 敏感字段，用于计算 CheckSum |
| `send_type` | select | 是 | `template` 通知短信（默认）/ `code` 验证码短信 |

**鉴权方式**：每个请求携带 Header `AppKey` + `Nonce` + `CurTime` + `CheckSum`，其中 `CheckSum = SHA1(AppSecret + Nonce + CurTime)`。

## 模板与变量

- **供应商原生模板**：必须配置模板（发送时读取 `TemplateCode`）。
- **占位符风格**：`{name}` 命名占位或 `%s` 匿名占位，**不可混用**；验证码模式（`send_type=code`）仅支持命名占位（codec `netease-native-v1`，允许命名 + 匿名 + Netease 双模式，见 `template_codec.go`）。
- **两种发送动作**：
  - `template` 通知短信 → `sendtemplate.action`，参数为顺序数组；
  - `code` 验证码短信 → `sendcode.action`，参数为 JSON map，且**仅支持单个手机号**。
- **签名**：系统侧必须绑定供应商签名（发送前校验），但签名不传给网易云信 API——签名在网易云信控制台与模板绑定。

## 发送与回执

- **批量**：模板短信单请求最多 100 个手机号，超出自动拆分多次请求；验证码模式批量时逐条发送。
- **回执关联**：同批共用一个 `sendid`，回执按 `(sendid, mobile)` 二元组关联到具体任务；验证码模式下 `sendid` 在 `msg` 字段、`obj` 为验证码本身。
- **回调**：`POST/GET /api/callback/:id`；请求体为 JSON，`eventType=11` 下行回执、`eventType=12` 上行回复；CheckSum 未强校验（服务端拿不到 AppSecret，采取宽松策略），响应 HTTP 200。

## 限制与注意事项

- **手机号格式**：中国大陆为裸 11 位号码；国际号码为 `+{国家码}-{号码}`，其他格式返回 414 错误。
- **无状态查询/拉取**：回执缺失时无法主动补齐，只能等回调或在上游排查。
- 验证码模式单号限制来自 `sendcode.action` API。

## 相关文件

- `modules/sender/infrastructure/netease_sms_sender.go` — 发送/批量/回调/鉴权
- `modules/sender/infrastructure/template_codec.go` — 占位符 codec
- `modules/sender/infrastructure/netease_sms_sender_test.go` — 测试

## 相关文档

- [供应商原生模板机制](../../provider-native-templates.md)
- [渠道总览](../README.md)
