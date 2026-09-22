# 掌榕网短信（zrwinfo_sms）

## 概述

掌榕网（1cloudsp，API 端点 `api.1cloudsp.com`）短信渠道，消息类型为 `sms`，provider code 为 `zrwinfo_sms`（`constants.ProviderZrwinfoSMS`）。实现位于 `modules/sender/infrastructure/`，是资源管理框架的参考实现（见 `docs/provider-resource-management.md`）。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | 多号码合并为一次请求 |
| 回调 `CallbackHandler` | ✅ | 不验签 |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ✅ | `POST /report/status` |
| 资源管理 | ✅ | 模板 + 签名完整 CRUD，签名支持 MatchAliases |
| 供应商签名 | 必须 | 系统自动补全 `【】` 括号 |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `accesskey` | text | 是 | — |
| `secret` | password | 是 | 敏感字段 |

## 模板与变量

- **供应商原生模板**：必须配置模板，发送前按模板归属账号校验。
- **占位符风格**：`{host_name}` 命名占位与 `{1}` 数字占位**均可，但不可混用**（codec `zrwinfo-native-v1`，允许命名 + 数字变量）。
- **发送内容**：变量值按 `##` 连接组成 `content`；每个变量 1–20 字符，且不得包含 `##` 或 `$$`（`zrwinfo_resource_content.go`）。
- **签名**：系统侧必须绑定供应商签名；缺失 `【】` 括号时自动补全。

## 发送与回执

- **单发**：`/api/v2/single_send`。
- **批量**：`/api/v2/send`，多个手机号逗号拼接为一次请求；回执 `ProviderID` 为 `{batchId}_{i}` 按序号关联。
- **回调**：`POST/GET /api/callback/:id`；请求体为 form-data（`smUuid`/`deliverResult`/`deliverTime`），不验签，响应 `{"code":"0","msg":"SUCCESS"}`。
- **状态拉取**：`POST /report/status`，已拉取的状态不重复返回；**需在掌榕网后台开启"主动获取"开关**，否则拉取不到数据。

## 限制与注意事项

- **仅支持中国大陆手机号**：非中国大陆号码直接判定失败并记录 callback_logs，错误码 `INVALID_RECEIVER`。
- 变量值长度与分隔符限制来自掌榕网 API（1–20 字符、禁 `##`/`$$`）。
- 状态拉取依赖上游开关，接入时先确认后台配置。

## 相关文件

- `modules/sender/infrastructure/zrwinfo_sms_sender.go` — 发送/批量/回调/状态拉取
- `modules/sender/infrastructure/zrwinfo_resources.go` — 模板与签名资源管理（参考实现）
- `modules/sender/infrastructure/zrwinfo_resource_content.go` — 变量内容校验
- `modules/sender/infrastructure/zrwinfo_resources_test.go` / `zrwinfo_named_placeholders_test.go` / `zrwinfo_sms_sender_test.go` — 测试

## 相关文档

- [供应商资源管理框架](../../provider-resource-management.md)（zrwinfo 为参考实现）
- [供应商原生模板机制](../../provider-native-templates.md)
- [渠道总览](../README.md)
