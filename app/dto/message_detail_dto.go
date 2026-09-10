package dto

import "cnb.cool/mliev/push/message-push/app/model"

const (
	MessageDetailSnapshot      = "snapshot"
	MessageDetailCurrentConfig = "current_config"
	MessageDetailUnavailable   = "unavailable"
)

type MessageDetail struct {
	model.SendSnapshot
	Source string `json:"source"`
	LogID  uint   `json:"log_id"`
}
