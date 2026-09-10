package worker

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
)

func newSendSnapshot(task *model.PushTask) *model.SendSnapshot {
	return &model.SendSnapshot{
		Version:        model.SendSnapshotVersion,
		CapturedAt:     timeutil.FormatRFC3339(timeutil.Now()),
		SignatureAlias: task.Signature,
		MessageContent: model.MessageContent{
			OriginalParamsRaw: task.TemplateParams,
			ContentType:       "text",
			UnavailableReason: "尚未完成消息准备",
		},
	}
}
