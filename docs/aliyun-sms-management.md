# 阿里云国内短信模板与签名管理

阿里云账号通过既有「供应商平台资源」入口管理模板和签名：查询列表、查看详情、新增、修改、删除，以及选择性导入本地。使用官方 Go SDK `dysmsapi-20170525/v5 v5.6.0`，端点固定为 `https://dysmsapi.aliyuncs.com`，复用账号的 `access_key_id` 和 `access_key_secret`。

## 接入范围与准备

- 管理国内验证码、通知和推广短信。国际模板不出现在管理列表中，按国际模板 Code 调用管理接口也会被拒绝。
- 在阿里云先准备审核通过的资质、授权委托书、商标及已上传的 OSS 材料。本系统接受对应 ID 和文件引用，不提供资质申请或文件上传流程。
- 签名来源支持企业和商标。签名类型新增时默认为通用，用途默认为自用；编辑时需明确选择签名类型，阿里云详情接口不返回该字段。签名来源同样需要明确选择，不从描述文本推测枚举。
- 材料引用采用 JSON 字符串数组，例如 `["1000000000000000/proof.png"]`。不接受临时下载 URL；每次编辑如需补充材料，应重新填写。材料引用和引流材料不进入镜像或诊断日志。

API 依据：[接口概览](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-overview)、[模板申请](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-createsmstemplate)、[签名申请](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-createsmssign)。

## RAM 权限

账号需要下列管理权限。原有发送、批量发送和发送状态查询继续使用原有权限；仅配置发送权限的 RAM 用户不能执行管理操作。

```json
{
  "Version": "1",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "dysms:QuerySmsTemplateList", "dysms:GetSmsTemplate",
      "dysms:CreateSmsTemplate", "dysms:UpdateSmsTemplate", "dysms:DeleteSmsTemplate",
      "dysms:QuerySmsSignList", "dysms:GetSmsSign",
      "dysms:CreateSmsSign", "dysms:UpdateSmsSign", "dysms:DeleteSmsSign"
    ],
    "Resource": "*"
  }]
}
```

## 管理接口

接口需要现有管理员认证，前缀为 `/api/admin/provider-accounts/:id`，`kind` 为 `templates` 或 `signatures`。

| 方法与路径 | 行为 |
| --- | --- |
| GET `/remote-resources/:kind` | 完整分页查询，每页 50 条；任何分页失败均不返回部分列表 |
| GET `/remote-resources/:kind/:remoteId` | 查询单条详情、当前版本和引用影响 |
| POST `/remote-resources/:kind` | 提交申请并保存本地镜像 |
| PUT `/remote-resources/:kind/:remoteId` | 修改，提交详情返回的 `version` 和 `confirm_impact` |
| DELETE `/remote-resources/:kind/:remoteId` | 删除，提交详情返回的 `version` 和 `confirm_impact` |
| POST `/resource-sync/preview` | 查询预览，体为 `{ "kind": "templates" }`，不写镜像 |
| POST `/resource-sync/import` | 按既有 selections/version 机制新增、更新、恢复或跳过 |
| POST `/template-parse` | 解析原生正文，不调用阿里云 |

模板的 `remoteId` 是 `TemplateCode`。签名的 `remoteId` 是签名名称 `SignName`，中文路径须 URL 编码；不是 `SignCode` 或工单号。已有手工维护的签名按同一账号的签名文本匹配，导入保留本地名称和启用开关。

编辑和删除前先查询详情，使用详情的版本进行提交。列表与详情字段不同，不能用列表版本直接修改资源。后台已自动执行该步骤。预览返回 `operation_errors` 提示操作限制，表单能力的 `read_only` 标识签名名称只读。

模板新增示例：

```json
{
  "name": "登录验证码",
  "content": "验证码为${code}，5分钟内有效。",
  "category": "0",
  "description": "用户登录验证",
  "provider_fields": {
    "related_sign_name": "木雷科技",
    "template_rule": "{\"code\":\"characterWithNumber\"}",
    "apply_scene_content": "已上线产品的用户登录页面"
  }
}
```

`category`：0 验证码、1 通知、2 推广。正文不超过 500 个字符，名称不超过 30 个字符。保留 `${变量名}` 原文，有变量时 `template_rule` 必须完整覆盖变量且无多余键，值为阿里云规则名称。推广模板需要 `more_data` 用户授权材料；含引流信息时通过 `traffic_driving` 传入阿里云规定的 JSON 对象数组。关联签名用于模板审核，发送时仍使用通道的签名映射。

签名新增示例：

```json
{
  "content": "木雷科技",
  "description": "用于用户登录和订单通知",
  "provider_fields": {
    "qualification_id": "123456789",
    "sign_source": "0",
    "sign_type": "1",
    "third_party": "false"
  }
}
```

`sign_source`：0 企业、5 商标；商标必须提供 `trademark_id`。`sign_type`：0 验证码、1 通用。`third_party=true` 时必须提供 `authorization_letter_id`。所有 ID 以字符串传递，后端按正整数解析，避免 JavaScript 大整数精度丢失。签名限 2～12 个中文、英文字母或数字，不能是纯数字；名称不可通过修改接口更改。

## 审核与恢复

模板仅审核未通过时允许修改。签名在审核通过或未通过时允许修改，修改后重新审核。审核中的模板和签名不能删除；取消审核的资源可删除，但不可用于发送。[模板修改限制](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-updatesmstemplate) · [签名修改限制](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-updatesmssign)

| 阿里云状态（详情 / 列表） | 本地 audit_status |
| --- | --- |
| 0 / AUDIT_STATE_INIT | 1，待审核 |
| 1 / AUDIT_STATE_PASS | 2，审核通过 |
| 2 / AUDIT_STATE_NOT_PASS | 3，审核未通过 |
| 10 / AUDIT_STATE_CANCEL（兼容 AUDIT_SATE_CANCEL） | 0，取消审核，不可用 |
| 缺失或未知值 | 0，待确认，不可用 |

`provider_metadata.aliyun` 保存审核原值、工单号、资质和运营商报备信息。报备状态独立展示：审核通过不意味着每个运营商均已完成报备；本次沿用现有审核准入规则。

修改和删除前先校验当前远端状态，再暂停已关联镜像。正文变化会清空通道参数映射，保存新正文时版本递增；只更新审核不会清空已确认映射。请求、查询预览和跳过不会直接恢复镜像的发送资格。

云端写入只执行一次。工单追踪标记保存在 `provider_metadata.pending_review`：正常受理后只有查询命中新工单才接受审核结果；响应丢失时，只有确认工单相对操作前发生变化才允许导入恢复。旧工单结果会显示同步阻塞，不会将镜像重新标为可用。

遇到超时、执行结果不确定或本地保存失败：先刷新供应商资源，必要时在阿里云控制台用工单号核对，待查询反映本次操作后选择更新/恢复同步；正文变更后重新配置通道参数映射。删除后远端资源不再出现在列表中，应核对本地引用并保持禁用。明确拒绝的请求不会留下等待新工单的阻塞，可重新查询导入恢复。错误响应保留供应商错误码、RequestId 和 `uncertain`，不自动重发写入。

## 部署与验证

复用既有镜像与元数据列，无新增数据库迁移。部署匹配的后端和管理前端；升级不会批量重写历史模板、签名或通道映射。测试使用模拟阿里云 HTTP 响应，不提交真实短信、模板或签名申请。

本次验证（2026-09-11）：

- `go test ./...` 和 `go vet ./...` 通过；供应商资源、发送器及服务层相关竞态测试通过。
- 管理前端 `apps/web-antd/src` 的 29 个测试文件、162 项测试通过；类型检查、修改文件的 ESLint 和 `build:antd` 构建通过。
- 仓库全部前端测试运行得到 65 个文件、463 项测试通过，另有安装页测试因既有 `/logo.svg` 导入无法解析而未能运行；本次没有修改安装页或其构建配置。共享资源加载测试还会输出 localhost:3000 连接失败日志，其断言通过。
