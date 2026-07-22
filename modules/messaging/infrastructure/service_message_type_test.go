package infrastructure

import (
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
)

func TestMessageServiceUsesSharedMessageTypeValidation(t *testing.T) {
	service := &MessageService{}
	for _, messageType := range []string{
		constants.MessageTypeSMS,
		constants.MessageTypeEmail,
		constants.MessageTypeWeChatWork,
		constants.MessageTypeDingTalk,
		constants.MessageTypeWebhook,
		constants.MessageTypePush,
	} {
		if !service.isValidMessageType(messageType) {
			t.Fatalf("shared valid message type %q was rejected", messageType)
		}
	}
	if service.isValidMessageType("unknown") {
		t.Fatal("unknown message type was accepted")
	}
}
