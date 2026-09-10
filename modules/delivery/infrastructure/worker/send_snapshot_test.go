package worker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/channel"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"cnb.cool/mliev/push/message-push/modules/sender"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	senderinfra "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	templateinfra "cnb.cool/mliev/push/message-push/modules/template/infrastructure"
)

func TestHandleStoresExactPreparedContentForSuccessAndMissingResponse(t *testing.T) {
	const provider = "send_snapshot_test_provider"
	if _, err := registry.GetByCode(provider); err != nil {
		if err := registry.Register(&registry.ProviderMeta{Code: provider, Name: "快照测试", Type: "sms", SupportsSend: true, TemplateCodec: senderinfra.NativeTemplateCodec{ID: "snapshot-native", AllowNamed: true}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, outcome := range []string{"success", "error_without_response", "nil_response", "invalidated"} {
		t.Run(outcome, func(t *testing.T) {
			db := newWorkerWebhookTestDB(t)
			task := createWorkerWebhookTask(t, db, "snapshot-"+outcome, "app")
			task.Status = "pending"
			task.MessageType = "sms"
			task.TemplateParams = `{"code":"086697","unused":"internal"}`
			if err := dao.NewPushTaskDAOWithDB(db).Update(task); err != nil {
				t.Fatal(err)
			}
			binding := &model.ChannelTemplateBinding{ID: 3, ChannelID: task.ChannelID, ProviderID: 7, ProviderTemplateID: 10, Status: 1, IsActive: 1, Weight: 10, MappedContentVersion: 1, Channel: &model.Channel{ID: task.ChannelID, Type: "sms", Status: 1, MessageTemplate: &model.MessageTemplate{Status: 1, Variables: `["code"]`}}, ParamMapping: `[{"type":"mapping","provider_var":"var1","system_var":"code"},{"type":"fixed","provider_var":"ttl","value":"5"}]`, ProviderTemplate: &model.ProviderTemplate{
				ID: 10, ProviderID: 7, ContentVersion: 1, Status: 1, ProviderAccount: &model.ProviderAccount{ID: 7, ProviderCode: provider, ProviderType: "sms", Status: 1}, TemplateCode: "16021", TemplateName: "登录验证码", TemplateContent: "验证码{var1}，{ttl}分钟有效。", ContentType: "text",
			}}
			cachedBinding := *binding
			cachedTemplate := *binding.ProviderTemplate
			cachedTemplate.TemplateContent = "旧缓存{var1}/{ttl}"
			cachedBinding.ProviderTemplate = &cachedTemplate
			if outcome == "invalidated" {
				binding.MappedContentVersion = 0
			}
			fake := &snapshotSender{code: provider, outcome: outcome}
			h := &MessageHandler{
				logger: noopLogger{}, taskDao: dao.NewPushTaskDAOWithDB(db), logDao: dao.NewPushLogDAOWithDB(db),
				templateHelper: templateinfra.NewTemplateHelper(), terminalService: service.NewTaskTerminalServiceWithDB(db),
				selector: snapshotSelector{node: &channel.ChannelNode{ChannelTemplateBinding: &cachedBinding, ProviderAccount: &model.ProviderAccount{
					ID: 7, AccountName: "发送时名称", ProviderCode: provider, Config: `{"secret":"must-not-be-copied"}`,
				}}},
				senderResolver: snapshotResolver{target: fake},
				loadBinding:    func(uint) (*model.ChannelTemplateBinding, error) { return binding, nil },
			}
			err := h.Handle(context.Background(), &queue.Message{Data: map[string]interface{}{"task_id": task.TaskID}})
			if (err != nil) != (outcome != "success") {
				t.Fatalf("Handle error=%v, outcome=%s", err, outcome)
			}
			logs, err := h.logDao.GetByTaskID(task.TaskID)
			if err != nil || len(logs) != 1 || logs[0].SendSnapshot == nil {
				t.Fatalf("logs=%+v error=%v", logs, err)
			}
			var snapshot model.SendSnapshot
			if err := json.Unmarshal([]byte(*logs[0].SendSnapshot), &snapshot); err != nil {
				t.Fatal(err)
			}
			if outcome == "invalidated" {
				if fake.calls != 0 || snapshot.UnavailableReason == "" {
					t.Fatal("invalidated mapping reached provider")
				}
				return
			}
			if snapshot.Content != fake.content || !reflect.DeepEqual(snapshot.MappedParams, fake.params) || snapshot.Content != "验证码086697，5分钟有效。" {
				t.Fatalf("snapshot differs from sender input: %+v, sender=%+v", snapshot, fake)
			}
			if snapshot.ProviderTemplateID != 10 || snapshot.ProviderName != "发送时名称" || snapshot.Version != 1 || snapshot.CapturedAt == "" || snapshot.UnavailableReason != "" {
				t.Fatalf("lost snapshot metadata: %+v", snapshot)
			}
			if strings.Contains(*logs[0].SendSnapshot, "must-not-be-copied") {
				t.Fatal("provider credentials leaked into snapshot")
			}
		})
	}
}

func TestPreparationFailureRecordsUnavailableAttempt(t *testing.T) {
	db := newWorkerWebhookTestDB(t)
	task := createWorkerWebhookTask(t, db, "snapshot-unavailable", "app")
	h := &MessageHandler{logger: noopLogger{}, taskDao: dao.NewPushTaskDAOWithDB(db), logDao: dao.NewPushLogDAOWithDB(db), terminalService: service.NewTaskTerminalServiceWithDB(db)}
	h.handleEarlyFailure(task, 0, "no available provider", newSendSnapshot(task))
	logs, err := h.logDao.GetByTaskID(task.TaskID)
	if err != nil || len(logs) != 1 || logs[0].SendSnapshot == nil {
		t.Fatalf("missing early failure snapshot: %+v %v", logs, err)
	}
	var snapshot model.SendSnapshot
	if err := json.Unmarshal([]byte(*logs[0].SendSnapshot), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot.UnavailableReason, "no available provider") || snapshot.Content != "" {
		t.Fatalf("wrong failure snapshot: %+v", snapshot)
	}
}

type snapshotSelector struct {
	noopSelector
	node *channel.ChannelNode
}

func (s snapshotSelector) SelectWithExcludes(context.Context, uint, string, string, string, []uint) (*channel.ChannelNode, error) {
	return s.node, nil
}

type snapshotResolver struct {
	sender.Resolver
	target sender.Sender
}

func (s snapshotResolver) GetSender(string) (sender.Sender, error) { return s.target, nil }

type snapshotSender struct {
	calls                  int
	code, outcome, content string
	params                 map[string]string
}

func (s *snapshotSender) GetProviderCode() string { return s.code }
func (s *snapshotSender) Send(_ context.Context, req *sender.SendRequest) (*sender.SendResponse, error) {
	s.calls++
	s.content, s.params = req.RenderedContent, req.MappedParams
	switch s.outcome {
	case "error_without_response":
		return nil, errors.New("provider timeout")
	case "nil_response":
		return nil, nil
	default:
		return &sender.SendResponse{Success: true, Status: "sent", ProviderID: "provider-message", RequestData: "{}", ResponseData: "{}"}, nil
	}
}
