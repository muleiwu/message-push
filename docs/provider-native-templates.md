# 供应商模板原文与参数映射

供应商模板由 `provider_id + template_code` 定位远端，本地通道通过 `provider_template_id` 引用本地记录。签名的远端 ID 与签名文本仍分别保存。

短信只以 `template_content` 原文作为解析依据。`variables` 和 `native_variables` 在响应中由供应商解析器生成，发送顺序不存入数据库。数字变量直接使用 `"1"`、`"2"`；命名变量保留原名。短信表单不提交独立变量定义。

`POST /api/admin/provider-accounts/:id/template-parse` 接收 `{ "content": "主机{1}" }`，返回原文、原生变量键、占位符和一次性的系统模板草稿。该操作无供应商网络调用，也不写数据库。原 `template-compile` 接口已移除。资源查询预览使用 `parsed`，不再返回 `compiled`。

模板 `content_version` 初始为 1。正文精确变化时，在事务中递增版本、清空所有引用绑定的 `param_mapping`，并将 `mapped_content_version` 归零。重复同步相同正文及修改审核、名称、备注不重置映射。查询、预览和跳过完全只读；跳过意味着本地配置可能仍与供应商不同。

绑定创建或确认映射时必须提交 `template_content_version` 和完整的 `param_mapping`。例如供应商 `{1}` 对应业务参数 `host_name`：

```json
{
  "template_content_version": 2,
  "param_mapping": [
    {"type": "mapping", "provider_var": "1", "system_var": "host_name", "value": ""}
  ]
}
```

过期版本返回 HTTP 409，用户需重新加载正文并配置。无变量模板也必须提交版本及空映射数组。空映射不再自动按同名变量发送。绑定响应的 `mapping_required` 表示是否需要重新确认。

供应商正文编辑或删除先使本地镜像和映射不可用，再调用供应商；超时或本地保存失败不会自动重发，请查询恢复后重新确认。已发送给供应商的请求不能撤回。

## 升级

三种数据库新增 `20260910000002_provider_native_templates.sql`。旧原生正文复制到 `template_content`，全部短信绑定（包括禁用和软删除的绑定）清空映射，非短信配置保留。重复有效模板代码阻止迁移，不自动合并记录。回滚不恢复旧映射，并禁用短信绑定。

安排短信维护窗口：暂停业务投递、处理存量任务、停止旧实例，部署后端及迁移，再部署管理前端。完成重新映射和测试后恢复短信投递。不要混用新旧后端；未确认绑定不会自动挂起排队任务等待配置。

## 验证记录（2026-09-10）

- Go 全量测试仅跳过下列三项已知统计失败，其余通过；相关包的竞态测试和 `go vet ./...` 通过。
- 管理前端 24 个测试文件、125 项测试通过；`@vben/web-antd` 类型检查通过，`build:antd` 构建退出码为 0。
- SQLite 执行了升级、回滚、唯一约束和重复数据预检测试；MySQL、PostgreSQL 的迁移完成同名及 SQL 契约检查，未连接实际数据库执行。
- 已知统计失败：`TestAdminStatisticsCountsTasksInsteadOfProviderAttempts`、`TestAdminStatisticsAppliesEachDimensionFilter`、`TestAdminStatisticsKeepsBucketsForDeletedResources`。
- 构建依赖 `tabs-ui` 的类型声明阶段输出 TS4058（`use-tabs-view-scroll` 引用 `scrollbar.vue` 的 `Props`）；构建仍成功，该依赖包未在本次修改。
