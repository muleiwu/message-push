# 消息渠道文档索引

每个消息渠道供应商（sender 实现）一篇说明文档。所有实现平铺在 `modules/sender/infrastructure/`（`package infrastructure`），在 `factory.go` 的 `NewFactory()` 中注册；可选能力（批量/回调/状态查询/状态拉取/资源管理）通过接口断言解析，见 `modules/sender/domain/sender.go` 与 `infrastructure/factory.go`。

## 渠道总览

| 渠道 | provider code | 消息类型 | 批量 | 回调 | 状态查询/拉取 | 资源管理 | 签名 |
|---|---|---|---|---|---|---|---|
| [阿里云短信](aliyun-sms/README.md) | `aliyun_sms` | sms | ✅ ≤1000 | ✅ | 查询 ✅ | 模板+签名 | 必须 |
| [腾讯云短信](tencent-sms/README.md) | `tencent_sms` | sms | ✅ ≤200 | ✅ | 查询 ✅ / 拉取 ✅ / 上行 ✅ | 模板+签名 | 必须 |
| [掌榕网短信](zrwinfo-sms/README.md) | `zrwinfo_sms` | sms | ✅ | ✅ | 拉取 ✅ | 模板+签名 | 必须 |
| [网易云信短信](netease-sms/README.md) | `netease_sms` | sms | ✅ ≤100 自动分批 | ✅ | ❌ | ❌ | 系统侧必须 |
| [邮件 SMTP](smtp/README.md) | `smtp` | email | ✅ 逐收件人 | ✅ 退信 | ❌ | ❌ | 必须（=标题） |
| [企业微信应用](wechat-work/README.md) | `wechat_work` | wechat_work | ✅ | ✅ | ❌ | ❌ | 无 |
| [企微群机器人](wechat-work-robot/README.md) | `wechat_work_robot` | wechat_work | ❌ | ❌ | ❌ | ❌ | 无 |
| [钉钉工作通知](dingtalk/README.md) | `dingtalk` | dingtalk | ✅ | ✅ 验签+解密 | ❌ | ❌ | 无 |
| [钉钉群机器人](dingtalk-robot/README.md) | `dingtalk_robot` | dingtalk | ❌ | ❌ | ❌ | ❌ | 无 |
| [QQ（OneBot 11）](onebot/README.md) | `onebot` | qq | ❌ | ❌ | ❌ | ❌ | 可选 |

## 通用机制

- **注册与解析**：sender 在 `modules/sender/infrastructure/factory.go` 注册，`GetSender(providerCode)` 按代码取用；`GetBatchSender()` / `GetCallbackHandler()` / `GetStatusQuerier()` / `GetStatusPuller()` 做能力断言。
- **账号配置字段**：定义在各 sender `init()` 的 `ProviderMeta.ConfigFields`，管理后台经 `GET /api/admin/provider-accounts/config-fields/:providerCode` 读取。
- **回调统一入口**：`POST/GET /api/callback/:id`（`id` = 供应商账号 ID），除钉钉（SHA1 验签 + AES 解密）外均不验签。
- **短信原生模板**：4 家短信供应商使用原生模板（`TemplateCode` + `TemplateContent` 解析占位符），机制见 [供应商原生模板](../provider-native-templates.md)；模板内容版本与映射确认版本不一致时拒绝发送。
- **资源管理**：阿里云/腾讯/掌榕网支持模板与签名的远程 CRUD，框架见 [供应商资源管理](../provider-resource-management.md)。

## 新增渠道

参见根目录 `CLAUDE.md` 的"Adding a New Message Provider"：在 `modules/sender/infrastructure/` 实现 `domain.Sender`（可选 `BatchSender` / `CallbackHandler` / `StatusQuerier` / `StatusPuller`），在 `factory.go` 注册，并在本目录补一篇 README。
