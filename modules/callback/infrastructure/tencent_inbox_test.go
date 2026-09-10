package infrastructure

import (
	"context"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/sender"
	senderinfra "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	"github.com/muleiwu/gsr"
)

type tencentInboxLogger struct{ gsr.Logger }

func (tencentInboxLogger) Error(string, ...gsr.LoggerField) {}

func TestTencentCallbackAcknowledgesOnlyPersistedInboxFacts(t *testing.T) {
	db := newCallbackServiceTestDB(t)
	if err := db.AutoMigrate(&model.ProviderAccount{}, &model.ProviderSMSEvent{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ProviderAccount{ID: 1, AccountCode: "tc", ProviderCode: "tencent_sms", ProviderType: "sms", Status: 1, Config: `{"sdk_app_id":"1400000001"}`}).Error; err != nil {
		t.Fatal(err)
	}
	s := &CallbackService{logger: tencentInboxLogger{}, senderResolver: senderinfra.NewFactory(), smsInbox: service.NewSMSEventServiceWithDB(db)}
	req := &sender.CallbackRequest{ProviderCode: "tencent_sms", ProviderAccountID: 1, RawBody: []byte(`[{"user_receive_time":"2026-09-10 18:00:00","nationcode":"86","mobile":"13800138000","report_status":"SUCCESS","errmsg":"DELIVRD","sid":"receipt-1"}]`)}
	for i := 0; i < 2; i++ {
		if response := s.HandleCallback(context.Background(), "tencent_sms", req); response.StatusCode != 200 {
			t.Fatalf("callback: %+v", response)
		}
	}
	var count int64
	db.Model(&model.ProviderSMSEvent{}).Count(&count)
	if count != 1 {
		t.Fatalf("callback duplicated inbox: %d", count)
	}
	var event model.ProviderSMSEvent
	db.First(&event)
	if event.Mobile != "+8613800138000" || event.ProviderAccountID != 1 || event.Source != "callback" || event.State != "pending" {
		t.Fatalf("inbox fields: %+v", event)
	}
	if err := db.Migrator().DropTable(&model.ProviderSMSEvent{}); err != nil {
		t.Fatal(err)
	}
	if response := s.HandleCallback(context.Background(), "tencent_sms", req); response.StatusCode != 500 {
		t.Fatalf("unpersisted callback acknowledged: %+v", response)
	}
}
