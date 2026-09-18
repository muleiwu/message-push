package helper

import (
	"fmt"
	"strconv"
	"strings"
)

// QQReceiver keeps the destination unambiguous across provider accounts.
type QQReceiver struct {
	MessageType string
	ID          int64
}

// ParseQQReceiver accepts private:<QQ number> or group:<group number>.
func ParseQQReceiver(receiver string) (QQReceiver, error) {
	kind, number, ok := strings.Cut(strings.TrimSpace(receiver), ":")
	if !ok || (kind != "private" && kind != "group") || number == "" {
		return QQReceiver{}, fmt.Errorf("QQ 接收者格式应为 private:QQ号 或 group:群号")
	}
	for _, ch := range number {
		if ch < '0' || ch > '9' {
			return QQReceiver{}, fmt.Errorf("QQ号或群号必须为正整数")
		}
	}
	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil || id <= 0 {
		return QQReceiver{}, fmt.Errorf("QQ号或群号必须为有效的 64 位正整数")
	}
	return QQReceiver{MessageType: kind, ID: id}, nil
}

type QQReceiverValidator struct{}

func (v *QQReceiverValidator) Validate(receiver string) error {
	_, err := ParseQQReceiver(receiver)
	return err
}

func (v *QQReceiverValidator) ValidateBatch(receivers []string) error {
	if len(receivers) == 0 {
		return fmt.Errorf("receivers cannot be empty")
	}
	for i, receiver := range receivers {
		if err := v.Validate(receiver); err != nil {
			return fmt.Errorf("receiver at index %d: %w", i, err)
		}
	}
	return nil
}
