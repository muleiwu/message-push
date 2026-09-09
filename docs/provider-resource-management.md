# 供应商资源管理与模板变量

资源操作按供应商代码注册，账号只提供凭据与本地归属。模板和签名分别声明新增、编辑、删除、查询；缺少处理器的操作不会出现在能力矩阵中，服务端也会拒绝执行。

## 代码扩展点

`modules/sender/domain/resource.go` 定义统一资源、操作、表单字段、协议映射和模板转换接口。供应商在 `ProviderMeta.Resources` 中注册 `ResourceDefinition`。每个操作包含：

- `Handler`：调用供应商后返回标准化 `RemoteResource`。写操作出现无法确认的结果时返回 `RemoteResourceError{Uncertain: true}`。
- `Protocol`：声明端点、规范字段到供应商请求字段的映射、响应字段和别名、成功码字段和值。
- `Fields`：声明本操作的用户表单字段、必填规则和选项。编辑接口不接受的字段不加入编辑声明。
- 模板的 `Codec`：提供带版本的 `Compile`、`Decode`、`Bind`，统一转换规范模板、供应商原生模板和发送参数位置。
- 可选 `MatchAliases`：只在首次关联本地签名时使用，声明供应商允许的签名表示形式。

注册函数会检查必要映射和处理器，供应商初始化必须处理注册错误。能力响应由注册项派生，不另外维护一份支持状态。`zrwinfo_resources.go` 是完整实现参考；`NamedTemplateCodec` 与 `PositionalTemplateCodec` 可分别复用到命名和位置变量的供应商。

## 模板与身份

统一模板使用 `{name}`。掌榕网编译示例：

```text
验证码{code}，{minutes}分钟有效，再次输入{code}
    ↓
验证码{1}，{2}分钟有效，再次输入{3}
位置映射：1 → code，2 → minutes，3 → code
发送值：123456##5##123456
```

本地保留规范内容、原生内容、变量位置映射、转换版本。首次导入只有位置编号的资源使用 `var1`、`var2` 等名称；存在旧映射时保留旧名称。通道的 `param_mapping` 仍将业务参数映射到这些规范变量，参数缺失或映射不完整会阻止发送。

掌榕网通过 `PositionalTemplateCodec{AllowNamed: true}` 同时接受原生数字占位符和 `{host_name}`、`{rule_name}` 等命名占位符。命名模板保留原变量名，发送按出现顺序取值，重复出现的变量重复取值；旧数字模板仍按编号和既有别名发送。数字与命名占位符混用会被明确拒绝。新增、编辑报备的数字转换规则保持兼容，存储版本仍为 `positional-v1`，无需数据库迁移。

资源预览的 `compiled.native_variables` 和本地模板响应的 `native_variables` 返回原文中的实际占位符（含括号），按首次出现顺序去重；它们用于展示，不用于决定发送参数顺序。空数组表示无变量，未提供或 `null` 表示解析信息不可用。列表展示供应商原文及这些变量标签，本地映射名称在详情中单独展示；例如供应商原文仍显示 `{1}`，对应的本地映射可以保持 `var1`。

模板 `template_code` 是供应商模板 ID；本地表主键不是供应商 ID。签名的 `remote_id` 是供应商签名 ID，`signature_code` 才是实际发送的完整签名。掌榕网新增/编辑读取 `ret`，查询/删除读取 `code`，各操作的 ID 字段分别声明。

## 管理接口

以下接口使用现有管理员认证，账号 ID 都指 `provider_accounts.id`。

| 路径 | 方法 | 行为 |
| --- | --- | --- |
| `/api/admin/provider-accounts/available` | GET | 返回各供应商 `resources` 能力与表单定义 |
| `/api/admin/provider-accounts/:id/remote-resources/:kind` | GET / POST | 查询远端 / 新增远端资源 |
| `/api/admin/provider-accounts/:id/remote-resources/:kind/:remoteId` | GET / PUT / DELETE | 远端详情 / 编辑 / 删除 |
| `/api/admin/provider-accounts/:id/template-compile` | POST | `{content}` 转换预览，不写入 |
| `/api/admin/provider-accounts/:id/resource-sync/preview` | POST | `{kind}` 准备差异、影响通道和版本，不写入 |
| `/api/admin/provider-accounts/:id/resource-sync/import` | POST | `{kind, selections:[{id, action, version}]}` 选择导入 |

`kind` 为 `templates` 或 `signatures`。规范新增/编辑字段为 `name`、`content`、`category`、`description`，具体必填字段取自能力声明。编辑/删除还需提交预览 `version`，有通道引用时提交 `confirm_impact: true`。远端删除 ID 使用路径参数，删除确认信息使用 JSON 请求体。

导入动作是 `create`、`update`、`restore`、`skip`。既有资源必须明确选动作，版本变化返回 HTTP 409。导入重新查询供应商，按账号串行并在事务中写入；保留本地名称、备注、启用开关、主键和引用。上游列表未出现的记录不会被自动删除。

只有部分操作的供应商可独立注册，不需要实现空方法。未注册查询时，准备接口仅展示本地已关联镜像供编辑/删除操作使用，界面明确说明来源，导入及远端查询不可用。

## 审核与失败语义

`audit_status = NULL` 表示历史手工资源，兼容旧发送逻辑；同步资源的 `0/1/2/3` 分别为待确认、待审核、通过、未通过。`remote_deleted` 独立记录远端删除。本地 `status` 仍只有启用/禁用；同步资源须审核通过、本地启用且未远端删除才可用于发送。

关联资源的内容通过供应商操作维护，本地编辑保留名称、备注和启用开关。远端删除保留本地镜像和引用供定位。审核、变量或内容变化会使相关通道重新校验，并清除选择器缓存。

供应商写操作只执行一次；超时不自动重试。远端成功但本地保存失败时，返回 `remote_id`、`pending_sync: true` 和提示，随后通过查询导入恢复本地镜像。此结果不表示远端操作已回滚。新增、编辑的确认响应也不等于审核通过，需要查询审核结果。

## 部署与验证

先部署后端，使自动迁移应用 `20260909000001_provider_resource_management.sql`，再部署前端。该迁移分别覆盖 MySQL、PostgreSQL 和 SQLite。既有手工记录无需批量转换；SMTP 标题继续使用本地管理。回滚迁移会停用未通过审核或已远端删除的资源，保留历史记录及原有本地禁用状态。

自动化测试通过模拟供应商接口验证协议、变量往返、并发导入、冲突、失败恢复、审核限制和界面交互，不会操作真实供应商账号。生产接入需要使用已配置且允许调用资源接口的掌榕网账号。
