# 阿里云短信（aliyun_sms）

## 概述

阿里云短信渠道，消息类型为 `sms`，provider code 为 `aliyun_sms`（`app/constants/message_type.go` 中 `constants.ProviderAliyunSMS`）。实现位于 `modules/sender/infrastructure/`，在 `factory.go` 的 `NewFactory()` 中注册。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | 单批最多 1000 个号码 |
| 回调 `CallbackHandler` | ✅ | 不验签 |
| 状态查询 `StatusQuerier` | ✅ | QuerySendDetails API |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ✅ | 模板 + 签名完整 CRUD（`aliyun_resources.go`） |
| 供应商签名 | 必须 | 发送时取签名的 `SignatureCode` |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `access_key_id` | text | 是 | 长度 16–64 |
| `access_key_secret` | password | 是 | 敏感字段，日志脱敏 |

字段定义在各 sender `init()` 的 `ProviderMeta.ConfigFields`，管理后台经 `GET /api/admin/provider-accounts/config-fields/:providerCode` 读取。

## 模板与变量

- **供应商原生模板**：必须配置 `template_code`（阿里云模板 ID），发送前按模板归属账号校验。
- **占位符风格**：`${name}` 命名占位（codec `aliyun-native-v1`，前缀 `$`、允许命名变量）。
- **变量映射门禁**：模板内容版本（`ContentVersion`）与绑定的映射确认版本（`MappedContentVersion`）不一致时拒绝发送（`infrastructure/sms_template.go`）。
- **签名**：系统侧必须为通道绑定供应商签名，发送时传 `SignatureCode`。

## 发送与回执

- **单发**：`BizId` 作为 `ProviderID` 关联回执。
- **批量**：一次请求最多 1000 个号码，全批共用同一份变量映射（`MappedParams`）；回执 `ProviderID` 为 `{bizId}_{i}` 按序号关联。
- **回调**：`POST/GET /api/callback/:id`（id=账号 ID）；请求体为 JSON 数组，不做验签，响应 `{"code":0,"msg":"接收成功"}`。
- **状态查询**：按 `BizId + 手机号 + 发送日期` 调用 QuerySendDetails；上游状态 1/2/3 分别映射为 pending/failed/delivered。

## 限制与注意事项

- 批量上限 1000（`modules/sender/domain/sender.go` 常量），且整批共享同一变量集 —— 逐号码变量需拆成多个任务。
- 资源管理（模板/签名同步）需要 RAM 账号具备相应权限，详见运维文档。
- 回调不验签：回调地址本身应视为半公开，依赖路径中的账号 ID 关联。

## 相关文件

- `modules/sender/infrastructure/aliyun_sms_sender.go` — 发送/批量/回调/状态查询
- `modules/sender/infrastructure/aliyun_client.go` — API 客户端与签名
- `modules/sender/infrastructure/aliyun_resources.go` / `aliyun_resource_query.go` / `aliyun_resource_mutation.go` — 模板与签名资源管理
- `modules/sender/infrastructure/aliyun_resources_test.go` — 测试

## 相关文档

- [阿里云短信管理](../../aliyun-sms-management.md) — 模板/签名 CRUD、RAM 权限、管理端点
- [供应商原生模板机制](../../provider-native-templates.md)
- [供应商资源管理框架](../../provider-resource-management.md)
- [渠道总览](../README.md)
