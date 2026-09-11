# OneBot 11：QQ 私聊与群通知

木雷消息服务通过 OneBot 11 的正向 HTTP API 发送消息。渠道类型为 `qq`，服务商代码为 `onebot`，管理后台显示为 **OneBot 11**。你需要先部署并登录可访问的 OneBot 11 实现，启用其 HTTP 服务。

## 配置账号和渠道

1. 在供应商账号中选择 **OneBot 11**，填写 HTTP 服务地址、可选的 Access Token 和消息格式。
2. 创建系统模板，例如正文 `{content}`，变量 `content`。
3. 在该账号下创建文本供应商模板，例如正文 `通知：{content}`，变量 `content`。模板代码使用本地唯一名称，例如 `qq-notification`，无需向 OneBot 注册模板。
4. 创建 **QQ** 渠道并选择系统模板，绑定供应商账号和供应商模板，将供应商变量 `content` 映射到系统变量 `content`。QQ 不要求签名或标题映射。
5. 在账号的“配置测试”中填写目标及正文并点击“发送测试消息”，验证账号；随后在渠道的“测试发送”中验证完整模板和队列链路，并查看任务结果。

账号配置示例：

```json
{
  "base_url": "http://127.0.0.1:5700",
  "access_token": "your-onebot-access-token",
  "message_format": "text"
}
```

| 字段 | 说明 |
| --- | --- |
| `base_url` | 必填 HTTP/HTTPS 服务地址；可带路径前缀，例如 `https://bot.example.com/onebot/`。不附加 `/send_msg`、查询参数、用户名密码或 URL 片段。容器部署时，该地址须能从消息服务容器访问。 |
| `access_token` | 可选；与 OneBot 配置一致。通过 `Authorization: Bearer …` 发送，不放入 URL 或发送日志。 |
| `message_format` | `text`（默认）或 `cqcode`。账号的发送和测试均采用此设置。 |

## 接收者与调用示例

继续调用 `POST /api/v1/messages`；以下展示请求正文，鉴权沿用现有 [API 接入说明](public-channel-catalog.md)。`channel_id` 替换为已配置的 QQ 渠道 ID。Go/PHP SDK 可直接传入字符串接收者，无需修改调用接口。

| 接收者 | 含义 |
| --- | --- |
| `private:123456` | 向 QQ 号 `123456` 发送私聊 |
| `group:987654` | 向 QQ 群 `987654` 发送群消息 |

每个接收者只包含一个目标。号码必须为 64 位正整数；不接受裸号码、负数、小数或逗号分隔的目标。一个渠道可同时用于私聊和群聊，切换账号不会改变目标含义。

私聊：

```json
{
  "channel_id": 42,
  "receiver": "private:123456",
  "template_params": { "content": "你的任务已完成。" }
}
```

群通知：

```json
{
  "channel_id": 42,
  "receiver": "group:987654",
  "template_params": { "content": "今晚 22:00 开始维护。" }
}
```

批量调用 `POST /api/v1/messages/batch`，逐条创建任务，所有目标共用模板参数：

```json
{
  "channel_id": 42,
  "receivers": ["private:123456", "group:987654"],
  "template_params": { "content": "服务已恢复。" }
}
```

## CQ 码

将账号的 `message_format` 设为 `cqcode`。例如以下参数在群里 @ 指定成员并发送图片：

```json
{
  "channel_id": 42,
  "receiver": "group:987654",
  "template_params": {
    "content": "[CQ:at,qq=123456] 请查看监控截图：[CQ:image,file=https://example.com/monitor.png]"
  }
}
```

`[CQ:at,qq=all]` 表示 @ 全体成员。CQ 码按原文交给 OneBot 解析，图片可达性、@ 权限和具体消息段能力由 OneBot 实现决定。CQ 参数中的特殊字符按 [OneBot 字符串消息规范](https://github.com/botuniverse/onebot-11/blob/master/message/string.md) 转义。

`text` 模式始终设置 `auto_escape=true`，上述 CQ 码会作为普通文字显示；`cqcode` 设置为 `false`。JSON 消息段数组、Markdown/HTML 渲染、事件接收、WebSocket、撤回和群管理暂未接入。普通文本和 CQ 码均存储为现有的文本模板。

## 结果和失败处理

消息服务先返回本地任务 ID，由 Worker 通过 JSON POST 调用 OneBot `/send_msg`，超时为 10 秒，并响应任务 context 的取消。只有 HTTP 200、`status=ok`、`retcode=0` 且返回有效的整数 `data.message_id` 时，任务才进入 `success`。消息 ID（包括负数和零）按字符串保存到发送记录；该结果表示 OneBot 报告发送成功，不提供已读或最终送达回执。

- OneBot 业务失败保留数字 `retcode` 作为错误码，优先使用实现返回的 `wording` / `message` 说明。
- HTTP 失败使用 `HTTP_401`、`HTTP_403` 等错误码；网络或超时使用 `ONEBOT_HTTP_ERROR`，缺失或异常响应使用 `ONEBOT_INVALID_RESPONSE`。非 JSON 响应以 `raw_body` 保存在日志中。
- 失败规则可选择 QQ 类型和 OneBot 服务商，按错误码配置重试、切换账号或失败处理。
- 本渠道调用同步接口。若 OneBot 返回异步受理状态，则使用 `ONEBOT_ASYNC_UNSUPPORTED`：无匹配规则时直接结束本地任务，不默认重试。上游可能已经发送，不能据此断言消息未发出；显式匹配的规则仍按管理员配置执行。
- 请求和响应诊断会移除当前账号的 Access Token；HTTP 重定向不自动跟随。

协议参考：[公开 API](https://github.com/botuniverse/onebot-11/blob/master/api/public.md)、[HTTP 通信](https://github.com/botuniverse/onebot-11/blob/master/communication/http.md)、[鉴权](https://github.com/botuniverse/onebot-11/blob/master/communication/authorization.md)。
