package helper

import (
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
)

func TestQQReceiverValidation(t *testing.T) {
	for _, tt := range []struct {
		receiver, kind string
		id             int64
	}{
		{"private:123456", "private", 123456},
		{" group:987654\n", "group", 987654},
		{"group:9223372036854775807", "group", 9223372036854775807},
		{"private:000123", "private", 123},
	} {
		got, err := ParseQQReceiver(tt.receiver)
		if err != nil || got.MessageType != tt.kind || got.ID != tt.id {
			t.Errorf("ParseQQReceiver(%q) = %+v, %v", tt.receiver, got, err)
		}
	}
	validator := GetReceiverValidator(constants.MessageTypeQQ)
	for _, receiver := range []string{"", "123456", "user:123", "Private:123", "group:", "group:0", "group:-1", "private:+1", "group:1.2", "group:1e3", "group:１２３", "group:12,34", "group:12:34", "group: 123", "group:9223372036854775808"} {
		if err := validator.Validate(receiver); err == nil {
			t.Errorf("accepted invalid receiver %q", receiver)
		}
		if err := validator.ValidateBatch([]string{"private:123456", receiver}); err == nil {
			t.Errorf("accepted batch containing %q", receiver)
		}
	}
	if err := validator.ValidateBatch(nil); err == nil {
		t.Fatal("accepted empty batch")
	}
	if err := validator.ValidateBatch([]string{"private:123456", "group:987654"}); err != nil {
		t.Fatal(err)
	}
}
