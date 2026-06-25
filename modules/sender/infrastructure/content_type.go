package infrastructure

import "cnb.cool/mliev/push/message-push/app/model"

// contentTypeMarkdown 供应商模板内容类型：Markdown
const contentTypeMarkdown = "markdown"

// isMarkdownContent 根据通道模板绑定的供应商模板内容类型判断是否为 Markdown。
// 内容格式由供应商模板的 ContentType 决定（text/html/markdown），
// 而非任务的 MessageType（后者是渠道类型，如 wechat_work/dingtalk）。
func isMarkdownContent(binding *model.ChannelTemplateBinding) bool {
	if binding == nil || binding.ProviderTemplate == nil {
		return false
	}
	return binding.ProviderTemplate.ContentType == contentTypeMarkdown
}
