# 邮件 SMTP（smtp）

## 概述

通用 SMTP 邮件渠道，消息类型为 `email`，provider code 为 `smtp`（`constants.ProviderSMTP`）。实现位于 `modules/sender/infrastructure/`。使用**系统模板**渲染（无供应商模板概念），供应商签名映射为**邮件标题**。

## 能力矩阵

| 能力 | 支持 | 说明 |
|---|---|---|
| 单发 `Sender` | ✅ | `Send()` |
| 批量 `BatchSender` | ✅ | 逐收件人循环单发（每人一封） |
| 回调 `CallbackHandler` | ✅ | 尽力解析退信，不验签 |
| 状态查询 `StatusQuerier` | ❌ | — |
| 状态拉取 `StatusPuller` | ❌ | — |
| 资源管理 | ❌ | — |
| 供应商签名 | 必须 | 签名即邮件标题（subject） |

## 账号配置字段

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `host` | text | 是 | SMTP 服务器 |
| `port` | number | 是 | 默认 587 |
| `username` | text | 是 | — |
| `password` | password | 是 | 敏感字段 |
| `from` | text | 是 | 发件人地址 |
| `encryption` | select | 是 | `none` / `starttls`（默认）/ `ssl` |

## 模板与变量

- **系统模板**：直接使用系统模板渲染结果（`RenderedContent`），无需 `template_code`。
- **内容类型**：模板 `ContentType=html` → `text/html`；`markdown` → 按纯文本发送。
- **签名 = 邮件标题**：`Signature.SignatureCode` 即 subject，必填且禁止包含 CR/LF。
- **附件**：base64 编码后以 MIME 附件发送（`smtp_mime.go`）。

## 发送与回执

- **单发/批量**：每个任务（每个收件人）单独发送一封邮件，不做多收件人合并。
- **回执**：SMTP 协议本身没有回执。`ProviderID` 合成为 `smtp_{taskID}`；发送成功即视为终态。
- **退信回调**：`POST/GET /api/callback/:id` 尽力解析两类内容——JSON 格式的 bounce webhook，或包含 `Delivery Status Notification` / `Undelivered Mail` 的原始退信邮件；不验签。

## 限制与注意事项

- **连接超时 30 秒**，慢速 SMTP 服务器会拖累 worker，注意规则引擎的重试配置。
- **附件配额**（应用层全局配置，`config/autoload/email_attachments.go`，管理端 `GET /api/admin/email-attachments/limits` 可查）：默认最多 5 个附件、单文件 5 MB、总计 10 MB，可用环境变量覆盖。
- 退信解析是尽力而为：依赖上游 MTA 投递退信到回调地址，不保证 100% 覆盖。

## 相关文件

- `modules/sender/infrastructure/smtp_sender.go` — 发送/批量/回调/退信解析
- `modules/sender/infrastructure/smtp_mime.go` — MIME 与附件编码
- `modules/sender/infrastructure/smtp_sender_subject_test.go` / `smtp_mime_test.go` — 测试

## 相关文档

- [渠道总览](../README.md)
