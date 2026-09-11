package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/migration"
	"cnb.cool/mliev/push/message-push/modules/channel"
	"cnb.cool/mliev/push/message-push/modules/delivery"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/worker"
	messaginginfra "cnb.cool/mliev/push/message-push/modules/messaging/infrastructure"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	ruleinfra "cnb.cool/mliev/push/message-push/modules/ruleengine/infrastructure"
	"cnb.cool/mliev/push/message-push/modules/sender"
	senderinfra "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	"cnb.cool/mliev/push/message-push/modules/template"
	templateinfra "cnb.cool/mliev/push/message-push/modules/template/infrastructure"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type oneBotTestConfig struct{ gsr.Provider }

func (oneBotTestConfig) GetInt(_ string, fallback int) int          { return fallback }
func (oneBotTestConfig) GetString(_ string, fallback string) string { return fallback }

type oneBotDeliveryLogger struct{ bindingControllerLogger }

func (oneBotDeliveryLogger) Warn(string, ...gsr.LoggerField) {}

type oneBotQueueRecorder struct {
	tasks   []*model.PushTask
	delayed int
}

func (p *oneBotQueueRecorder) Push(_ context.Context, task *model.PushTask) error {
	p.tasks = append(p.tasks, task)
	return nil
}
func (p *oneBotQueueRecorder) PushBatch(_ context.Context, tasks []*model.PushTask) error {
	p.tasks = append(p.tasks, tasks...)
	return nil
}
func (p *oneBotQueueRecorder) PushDelayed(context.Context, *model.PushTask, time.Time) error {
	p.delayed++
	return nil
}

type oneBotTestSelector struct {
	channel.Selector
	bindingID uint
}

func (s oneBotTestSelector) SelectWithExcludes(context.Context, uint, string, string, string, []uint) (*channel.ChannelNode, error) {
	binding, err := dao.NewChannelTemplateBindingDAO().GetByID(s.bindingID)
	if err != nil {
		return nil, err
	}
	return &channel.ChannelNode{ChannelTemplateBinding: binding, ProviderAccount: binding.ProviderTemplate.ProviderAccount}, nil
}
func (oneBotTestSelector) ReportSuccess(uint) {}
func (oneBotTestSelector) ReportFailure(uint) {}

// This runs the public message service, template binding and real worker against
// the migrated schema. Only the Redis transport and remote OneBot are replaced.
func TestOneBotAccountAndQueuedDelivery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "onebot.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := migration.RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DELETE FROM failure_rules").Error; err != nil {
		t.Fatal(err)
	}
	registerBindingControllerDependency(db)
	registerBindingControllerDependency[gsr.Logger](oneBotDeliveryLogger{})
	registerBindingControllerDependency[gsr.Provider](oneBotTestConfig{})
	registerBindingControllerDependency[gsr.Enver](oneBotTestConfig{})
	registerBindingControllerDependency[sender.Resolver](senderinfra.NewFactory())
	registerBindingControllerDependency[template.Renderer](templateinfra.NewTemplateHelper())
	producer := &oneBotQueueRecorder{}
	registerBindingControllerDependency[delivery.Producer](producer)
	registerBindingControllerDependency[ruleengine.Engine](ruleinfra.New())

	var async atomic.Bool
	var calls atomic.Int32
	bodies := make(chan map[string]any, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		bodies <- body
		if async.Load() {
			fmt.Fprint(w, `{"status":"async","retcode":1,"data":null}`)
		} else {
			fmt.Fprint(w, `{"status":"ok","retcode":0,"data":{"message_id":-42}}`)
		}
	}))
	defer server.Close()
	accounts := service.NewAdminProviderAccountService()
	account, err := accounts.CreateProviderAccount(nil, &dto.CreateProviderAccountRequest{Name: "QQ 测试账号", ProviderCode: constants.ProviderOneBot, Config: map[string]any{"base_url": server.URL, "message_format": "cqcode"}})
	if err != nil || account.ProviderType != constants.MessageTypeQQ {
		t.Fatalf("account=%+v err=%v", account, err)
	}
	for _, req := range []*dto.TestProviderRequest{{Message: "hello"}, {Receiver: "group:0", Message: "hello"}, {Receiver: "private:123", Message: " "}} {
		if _, err := accounts.TestProviderAccount(account.ID, req); err == nil {
			t.Fatal("invalid account test accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid account test sent a message")
	}
	testResult, err := accounts.TestProviderAccount(account.ID, &dto.TestProviderRequest{Receiver: "private:123", Message: "账号测试"})
	if err != nil || !testResult.Success {
		t.Fatalf("test result=%+v err=%v", testResult, err)
	}
	if body := <-bodies; body["message"] != "账号测试" || body["user_id"] != float64(123) {
		t.Fatalf("account test payload=%v", body)
	}

	systemTemplate := &model.MessageTemplate{TemplateName: "QQ 通知", Content: "通知 {content}", ContentType: "text", Variables: `["content"]`, Status: 1}
	if err := db.Create(systemTemplate).Error; err != nil {
		t.Fatal(err)
	}
	qqChannel := &model.Channel{Name: "QQ 通知", Type: constants.MessageTypeQQ, MessageTemplateID: systemTemplate.ID, Status: 1}
	if err := db.Create(qqChannel).Error; err != nil {
		t.Fatal(err)
	}
	providerTemplate := &model.ProviderTemplate{ProviderID: account.ID, TemplateCode: "qq-notification", TemplateName: "QQ 通知模板", TemplateContent: "提醒：{body}", ContentType: "text", Variables: `["body"]`, Status: 1}
	if err := db.Create(providerTemplate).Error; err != nil {
		t.Fatal(err)
	}
	binding := &model.ChannelTemplateBinding{ChannelID: qqChannel.ID, ProviderID: account.ID, ProviderTemplateID: providerTemplate.ID, ParamMapping: `[{"type":"mapping","provider_var":"body","system_var":"content"}]`, Weight: 10, Priority: 100, Status: 1, IsActive: 1}
	if err := db.Create(binding).Error; err != nil {
		t.Fatal(err)
	}
	registerBindingControllerDependency[channel.Selector](oneBotTestSelector{bindingID: binding.ID})
	messages := messaginginfra.NewMessageService()
	params := map[string]string{"content": "你好\n[CQ:at,qq=123]"}
	ctx := context.Background()
	for _, receiver := range []string{"123", "group:0", "private:-1"} {
		if _, err := messages.Send(ctx, &dto.SendRequest{AppID: "qq-app", ChannelID: qqChannel.ID, Receiver: receiver, TemplateParams: params}); err == nil {
			t.Fatal("invalid receiver was queued")
		}
	}
	if _, err := messages.BatchSend(ctx, &dto.BatchSendRequest{AppID: "qq-app", ChannelID: qqChannel.ID, Receivers: []string{"private:123", "group:0"}, TemplateParams: params}); err == nil {
		t.Fatal("invalid batch was queued")
	}
	var count int64
	db.Model(&model.PushTask{}).Count(&count)
	if count != 0 || len(producer.tasks) != 0 {
		t.Fatal("invalid receivers created tasks")
	}
	if _, err := messages.Send(ctx, &dto.SendRequest{AppID: "qq-app", ChannelID: qqChannel.ID, Receiver: "private:123", TemplateParams: params}); err != nil {
		t.Fatal(err)
	}
	batch, err := messages.BatchSend(ctx, &dto.BatchSendRequest{AppID: "qq-app", ChannelID: qqChannel.ID, Receivers: []string{"group:456", "private:789"}, TemplateParams: params})
	if err != nil || batch.SuccessCount != 2 || len(producer.tasks) != 3 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	handler := worker.NewMessageHandler()
	for _, task := range producer.tasks {
		message := &queue.Message{Data: map[string]any{"task_id": task.TaskID}}
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		body := <-bodies
		if body["message"] != "提醒："+params["content"] || body["auto_escape"] != false {
			t.Fatalf("template or format was lost: %v", body)
		}
		var saved model.PushTask
		if err := db.Where("task_id = ?", task.TaskID).First(&saved).Error; err != nil || saved.Status != constants.TaskStatusSuccess {
			t.Fatalf("saved=%+v err=%v", saved, err)
		}
		var log model.PushLog
		if err := db.Where("task_id = ?", task.TaskID).First(&log).Error; err != nil || log.ProviderMsgID != "-42" || log.SendSnapshot == nil || !strings.Contains(*log.SendSnapshot, "提醒") {
			t.Fatalf("log=%+v err=%v", log, err)
		}
		// A duplicate queue entry must not trigger another HTTP send.
		if err := handler.Handle(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 4 {
		t.Fatal("unexpected repeated send")
	}
	async.Store(true)
	accepted, err := messages.Send(ctx, &dto.SendRequest{AppID: "qq-app", ChannelID: qqChannel.ID, Receiver: "group:456", TemplateParams: params})
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(ctx, &queue.Message{Data: map[string]any{"task_id": accepted.TaskID}}); err != nil {
		t.Fatal(err)
	}
	var failed model.PushTask
	if err := db.Where("task_id = ?", accepted.TaskID).First(&failed).Error; err != nil || failed.Status != constants.TaskStatusFailed || failed.RetryCount != 0 || producer.delayed != 0 {
		t.Fatalf("async result=%+v delayed=%d err=%v", failed, producer.delayed, err)
	}
}
