# 腾讯云国内短信管理

管理后台的腾讯云账号提供「短信状态与回复」入口。模板、签名仍在既有供应商资源管理中操作。开发测试只使用模拟腾讯云响应；自动轮询默认关闭。

## 支持范围

使用 `2021-01-11` API，`sms` 和 `common` SDK 固定为 `v1.3.172`。服务端使用账号现有的 SecretId、SecretKey、SmsSdkAppId 和地域配置；地域缺省为 `ap-guangzhou`，端点固定为 `sms.tencentcloudapi.com`。

| 资源 | API |
| --- | --- |
| 模板 | AddSmsTemplate、ModifySmsTemplate、DescribeSmsTemplateList、DeleteSmsTemplate |
| 签名 | AddSmsSign、ModifySmsSign、DescribeSmsSignList、DeleteSmsSign |
| 下发回执 | PullSmsSendStatusByPhoneNumber、PullSmsSendStatus |
| 上行回复 | PullSmsReplyStatusByPhoneNumber、PullSmsReplyStatus |

管理操作限定国内短信，相关请求固定 `International=0`。删除接口不接受此参数，因此先查询确认国内资源身份。既有短信发送能力和其他供应商不受此管理范围影响。

## 模板与签名

模板保存供应商原文，使用 `{1}`、`{2}` 等数字变量。业务参数的对应关系在通道中配置；不自动改写原文。短信类型使用腾讯云定义：营销 1、通知 2、验证码 3。腾讯云查询不返回短信类型，编辑时需要明确选择。

列表查询每页请求 100 条并完整翻页，中途失败不返回部分列表。资源查询、预览不会写入镜像，导入仍需选择新增、更新、恢复或跳过。正文变化沿用现有版本递增和映射重置规则，审核状态更新不重置映射。

签名提交增加 `provider_fields`：

```json
{
  "content": "公司签名",
  "description": "申请说明",
  "provider_fields": {
    "sign_type": "0",
    "document_type": "1",
    "sign_purpose": "0",
    "qualification_id": "1000001",
    "proof_image": "不含 data 前缀的 Base64"
  }
}
```

公司允许证明类型 0/1，商标允许 7，机构允许 2/3；国内不提供 APP、网站、公众号、小程序签名类型。他用签名还必须提交 `commission_image`。图片为 JPEG/PNG，前端限制原文件 6 MiB，后端限制 Base64 长度 8 MiB；图片不进入本地镜像、预览响应、版本摘要或错误日志。编辑时重新上传材料。

腾讯云原始审核码保存在 `provider_metadata.status_code`，独立于系统内部状态：

| 腾讯云 | 系统 audit_status | 展示 |
| --- | --- | --- |
| 0 | 2 | 审核通过、可用 |
| 1 | 1 | 待审核 |
| 2 | 1 | 审核通过待生效，仍不可发送 |
| -1 | 3 | 审核未通过 |
| 未知/缺失 | 0 | 待确认 |

签名查询同时保留资质 ID、名称及资质状态。远端写入只执行一次，不以受理响应代替审核结果；错误响应保留腾讯云错误码、RequestId 和是否结果不确定。

## 状态与回复接口

以下接口均使用现有管理员认证，路径前缀为 `/api/admin/provider-accounts/:id`，`kind` 是 `reports` 或 `replies`。

| 方法与路径 | 用途 |
| --- | --- |
| POST `/sms-events/:kind/query` | 查询单号最近七天数据，并补入本地 |
| POST `/sms-events/:kind/pull` | 消费一批队列数据并保存 |
| GET `/sms-events` | 读取本地事件，支持分页和 kind/state/source/mobile/attribution 筛选 |
| POST `/sms-events/:eventId/retry` | 重试已保存但处理失败的事件，不调用腾讯云 |
| GET `/sms-polling` | 读取轮询设置和最近 20 次执行记录，不初始化数据库记录 |
| PUT `/sms-polling` | 更新轮询设置，或显式恢复暂停的流 |

单号查询体为 `{phone_number, begin_time, end_time?, limit?}`。时间使用 Unix 秒，结束时间默认当前时间，最多查询七天；手机号规范化为国内 E.164 格式。`limit` 默认 100，范围 1～100，Offset 固定为 0。返回 `limit_reached` 时仅表示达到本次上限，不能证明已取完全部历史数据。

批量拉取体为 `{limit?:100}`。返回 `run_id`、`request_id`、`received`、`inserted`、`duplicates`、`pending`、`limit_reached` 和本地事件记录。成功表示已保存；任务和通知由本地处理器异步处理。

轮询配置为 `{reports_enabled, replies_enabled, interval_seconds, resume_reports?, resume_replies?}`。两个开关默认 false，间隔默认 30 秒、允许 10～3600 秒；保存间隔不自动清除暂停原因，恢复需显式设置对应的 resume 字段。

## 持久化、去重与归属

- 每次云端读取先创建执行记录；响应与收件事件在事务中落库，然后才执行本地业务动作。
- 下发回执的幂等键由 SmsSdkAppId、类型、流水号、号码、投递结果构成。腾讯云下发回调走同一入口，国内回调保存失败时不返回成功确认。
- 回复的幂等键由 SmsSdkAppId、类型、号码、回复时间、内容、签名、扩展码构成。腾讯云没有独立回复 ID，因此这些字段完全相同的回复被视为重复。
- 回执按本地账号、流水号和等价号码格式关联发送记录。旧发送尝试的迟到回执不会覆盖更新的发送尝试。
- 回复仅匹配同一账号、同一号码、回复前七天、实际发送签名一致的记录。候选只对应一个应用时转发；无候选标记 unmatched，多应用标记 ambiguous，均保留回复且不广播。
- 回调日志、任务状态、规则决策和 Webhook Outbox 在本地事务中完成。重试/切换产生的队列动作持久化到事件，并使用稳定事件键在 Redis 原子入队；重放不会再次增加重试次数。
- 未知投递结果、缺失身份或不支持的号码保留原始响应并标记需核对，不直接作为发送失败处理。

事件处理失败只重试本地记录，采用有界退避。已有发送任务的终态和 Webhook Outbox 幂等规则继续生效。Redis 中 `push:sms-effect:*` 保存投递动作幂等标记，不自动过期；当前未增加事件或幂等标记的自动清理任务。

## 自动轮询与故障恢复

手动和自动拉取按 SmsSdkAppId、类型互斥。同一队列最多由一个本地账号启用自动轮询。每轮最多五批，每批 100 条，执行期限 45 秒、锁期限 60 秒；满批时尽快继续，空队列按配置间隔等待。

账号禁用、删除或配置改变后暂停轮询。凭据、权限错误和消费结果不确定时暂停对应流，并显示原因。应用重启后，超过锁期限仍未完成的执行记录标记为结果不确定，不自动再次消费；核对执行记录、按号码补录后可显式恢复。

腾讯云两个批量接口需额外开通且只允许消费一次。云端消费与本地提交无法构成跨系统事务：进程可能在收到响应前中断，或本地保存失败。因此不承诺零丢失，也不通过自动重发消费请求掩盖不确定结果。

## 部署与验证

先运行三个数据库对应的 `20260910000003_tencent_sms_management.sql` 自动迁移，再部署匹配的后端和管理前端。迁移添加资源元数据、回调账号/来源字段，以及事件、执行记录和轮询设置表；不清空历史记录、模板正文或通道映射。回滚只删除本次新增结构，回滚前先关闭轮询并处理待办事件。

实施前基线：前端 24 个文件、125 项测试通过；Go 全量测试存在三组已有统计失败：`TestAdminStatisticsCountsTasksInsteadOfProviderAttempts`、`TestAdminStatisticsAppliesEachDimensionFilter`、`TestAdminStatisticsKeepsBucketsForDeletedResources`。本次验证须将这些基线失败单列。

本次验证（2026-09-10）：

- 排除上述三组基线统计失败后，Go 全量测试通过；腾讯云接口、持久化处理、回调与队列的竞态测试和 `go vet ./...` 通过。
- 前端 27 个测试文件、136 项测试通过；类型检查、修改文件的 ESLint 和 `build:antd` 构建通过。
- SQLite 完整升级/回滚、历史映射保留和唯一队列拥有者测试通过；MySQL、PostgreSQL 完成同名迁移及 SQL 结构检查，未连接真实数据库执行迁移。
- 隔离 Redis 验证了投递动作被消费后重放不会再次入队。浏览器使用模拟接口验证了桌面、390px 窄屏、深色模式和签名材料表单，打开/刷新不会消费腾讯云队列。
