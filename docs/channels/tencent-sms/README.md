# 腾讯云短信（tencent_sms）

## 概述

腾讯云短信渠道，消息类型为 `sms`，provider code 为 `tencent_sms`（`constants.ProviderTencentSMS`）。实现位于 `modules/sender/infrastructure/`，是唯一同时支持状态查询、状态拉取和上行短信的供应商。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | 单批最多 200 个号码 |
| 回调 `CallbackHandler` | ✅ | 不验签 |
| 状态查询 `StatusQuerier` | ✅ | PullSmsSendStatusByPhoneNumber |
| 状态拉取 `StatusPuller` | ✅ | PullSmsSendStatus（消费式队列） |
| 上行短信 `SMSReporter` | ✅ | QuerySMSEvents / PullSMSEvents（`tencent_events.go`） |
| 资源管理 | ✅ | 模板 + 签名完整 CRUD（仅国内短信） |
| 供应商签名 | 必须 | 发送时传签名内容 |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `secret_id` | text | 是 | — |
| `secret_key` | password | 是 | 敏感字段 |
| `sdk_app_id` | text | 是 | 短信应用 ID |
| `region` | text | 否 | 默认 `ap-guangzhou` |

## 模板与变量

- **供应商原生模板**：必须配置 `template_id`，发送前按模板归属账号校验。
- **占位符风格**：`{1}` `{2}` 纯数字序号占位（codec `tencent-native-v1`，仅允许数字变量）；参数按顺序数组（`TemplateParamSet`）传递。
- **变量映射门禁**：同其他短信渠道，`ContentVersion` / `MappedContentVersion` 不一致时拒绝发送。
- **签名**：系统侧必须绑定供应商签名。

## 发送与回执

- **单发/批量**：`TemplateParamSet` 取绑定变量的有序值；批量一次请求最多 200 个号码。
- **回调**：`POST/GET /api/callback/:id`；请求体为 JSON 数组（`sid`/`mobile`/`report_status`），不验签，响应 `{"result":0,"errmsg":"OK"}`。
- **状态查询**：按手机号拉取 `PullSmsSendStatusByPhoneNumber`——仅能查最近 7 天、仅支持中国大陆手机号、单次 limit 1–100。
- **状态拉取**：`PullSmsSendStatus` 为消费式队列，已拉取的状态不会重复返回，适合定时轮询补齐回执（`app/service/sms_polling_service.go`）。

## 限制与注意事项

- 批量上限 200（`modules/sender/domain/sender.go` 常量）。
- 状态查询的时间窗（7 天）与地域（中国大陆）限制来自腾讯云 API 本身。
- 资源管理仅覆盖国内短信（`International=0`），国际短信模板/签名需在腾讯云控制台维护。

## 相关文件

- `modules/sender/infrastructure/tencent_sms_sender.go` — 发送/批量/回调/状态查询
- `modules/sender/infrastructure/tencent_client.go` — API 客户端与签名
- `modules/sender/infrastructure/tencent_events.go` — 状态拉取与上行短信
- `modules/sender/infrastructure/tencent_resources.go` — 模板与签名资源管理
- `modules/sender/infrastructure/tencent_management_test.go` — 测试

## 相关文档

- [腾讯云短信管理](../../tencent-sms-management.md) — 模板/签名 API、状态拉取、上行短信、轮询
- [供应商原生模板机制](../../provider-native-templates.md)
- [供应商资源管理框架](../../provider-resource-management.md)
- [渠道总览](../README.md)
