package service

import (
	"encoding/json"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	templateinfra "cnb.cool/mliev/push/message-push/modules/template/infrastructure"
	"gorm.io/gorm"
)

func newMessageDetailTestService(t *testing.T) (*AdminMessageDetailService, *gorm.DB, *model.PushTask) {
	t.Helper()
	db := newAdminTaskTestDB(t)
	for _, sql := range []string{
		`CREATE TABLE callback_logs (provider_account_id INTEGER DEFAULT 0, source TEXT, sms_event_key TEXT UNIQUE, attribution TEXT, id INTEGER PRIMARY KEY, task_id TEXT, raw_data TEXT)`,
		`CREATE TABLE provider_templates (provider_metadata TEXT, content_version INTEGER NOT NULL DEFAULT 1, id INTEGER PRIMARY KEY, provider_id INTEGER, template_code TEXT, template_name TEXT, template_content TEXT, content_type TEXT, variables TEXT, deleted_at DATETIME)`,
		`CREATE TABLE channel_template_bindings (mapped_content_version INTEGER NOT NULL DEFAULT 0, id INTEGER PRIMARY KEY, channel_id INTEGER, provider_id INTEGER, provider_template_id INTEGER, param_mapping TEXT, deleted_at DATETIME)`,
		`INSERT INTO provider_accounts (id, account_name, provider_code) VALUES (7, '旧供应商', 'netease_sms')`,
		`INSERT INTO channels (id, name) VALUES (1, '验证码通道')`,
		`INSERT INTO provider_templates (id, provider_id, template_code, template_name, template_content, content_type, variables, deleted_at) VALUES (10, 7, '16021', '验证码模板', '您的验证码为{var1}，{ttl}分钟内有效。', 'text', '["var1","ttl"]', NULL)`,
		`INSERT INTO channel_template_bindings (id, channel_id, provider_id, provider_template_id, param_mapping, deleted_at) VALUES (3, 1, 7, 10, '[{"type":"mapping","provider_var":"var1","system_var":"code"},{"type":"fixed","provider_var":"ttl","value":"5"}]', NULL)`,
		`INSERT INTO push_tasks (id, task_id, app_id, channel_id, provider_account_id, message_type, receiver, template_params, status) VALUES (1, 'detail-task', 'app', 1, 7, 'sms', '13800138000', '{"code":"086697"}', 'success')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	task, err := dao.NewPushTaskDAOWithDB(db).GetByID(1)
	if err != nil {
		t.Fatal(err)
	}
	return NewAdminMessageDetailService(db, templateinfra.NewTemplateHelper()), db, task
}

func TestLegacyCallbackActionDoesNotReplaceLastSendAttempt(t *testing.T) {
	s, db, task := newMessageDetailTestService(t)
	callbackBody := `{"provider_id":"message-1","status":"failed"}`
	if err := db.Exec(`INSERT INTO callback_logs (id, task_id, raw_data) VALUES (1, ?, ?)`, task.TaskID, callbackBody).Error; err != nil {
		t.Fatal(err)
	}
	logs := []*model.PushLog{
		{ID: 2, TaskID: task.TaskID, ProviderAccountID: 7, RequestData: callbackBody},
		{ID: 1, TaskID: task.TaskID, ProviderAccountID: 7, RequestData: `{"templateid":"16021"}`},
	}
	got := s.Latest(task, logs)
	if got.LogID != 1 || got.Source != dto.MessageDetailCurrentConfig {
		t.Fatalf("callback replaced the old send: %+v", got)
	}
	perLog := s.ForLogs(task, logs)
	if perLog[2].Source != dto.MessageDetailUnavailable || perLog[1].Content != got.Content {
		t.Fatalf("callback was treated as a send attempt: %+v", perLog)
	}
}

func TestLegacyMessageDetailOnlyPreviewsUniqueCurrentTemplate(t *testing.T) {
	s, db, task := newMessageDetailTestService(t)
	log := &model.PushLog{ID: 20, ProviderAccountID: 7, RequestData: `{"templateid":"16021"}`}
	got := s.Latest(task, []*model.PushLog{log})
	if got.Source != dto.MessageDetailCurrentConfig || got.Content != "您的验证码为086697，5分钟内有效。" || len(got.MappingDetails) != 2 {
		t.Fatalf("unexpected legacy preview: %+v", got)
	}
	// A second template on the same provider must not be chosen by priority.
	for _, sql := range []string{
		`INSERT INTO provider_templates (id, provider_id, template_code, template_name, template_content, content_type, variables, deleted_at) VALUES (11, 7, 'another', '另一个模板', '另一内容{code}', 'text', '["code"]', NULL)`,
		`INSERT INTO channel_template_bindings (id, channel_id, provider_id, provider_template_id, param_mapping, deleted_at) VALUES (4, 1, 7, 11, '[]', NULL)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	got = s.Latest(task, []*model.PushLog{log})
	if got.ProviderTemplateID != 10 {
		t.Fatalf("request template ID was ignored: %+v", got)
	}
	log.RequestData = `{}`
	if got = s.Latest(task, []*model.PushLog{log}); got.Source != dto.MessageDetailUnavailable || !strings.Contains(got.UnavailableReason, "多个") {
		t.Fatalf("ambiguous legacy record was rendered: %+v", got)
	}
	log.RequestData = `{"templateid":"16021"}`
	if err := db.Exec(`UPDATE provider_templates SET deleted_at = CURRENT_TIMESTAMP WHERE id = 10`).Error; err != nil {
		t.Fatal(err)
	}
	if got = s.Latest(task, []*model.PushLog{log}); got.Source != dto.MessageDetailUnavailable {
		t.Fatalf("deleted template was rendered: %+v", got)
	}
}

func TestSavedMessageDetailsSurviveConfigurationChangesAndCallbacks(t *testing.T) {
	s, db, task := newMessageDetailTestService(t)
	logs := dao.NewPushLogDAOWithDB(db)
	first := &model.SendSnapshot{Version: 1, ProviderAccountID: 7, ProviderName: "首次发送供应商", MessageContent: model.MessageContent{Content: "第一次086697"}}
	second := &model.SendSnapshot{Version: 1, ProviderAccountID: 8, ProviderName: "切换后供应商", SignatureAlias: "登录", SignatureValue: "已映射签名", MessageContent: model.MessageContent{Content: "第二次086697", OriginalParams: map[string]string{"code": "086697"}}}
	for _, snapshot := range []*model.SendSnapshot{first, second, nil} {
		if err := logs.Create(&model.PushLog{TaskID: task.TaskID, AppID: task.AppID, ProviderAccountID: 7, Status: "failed", RequestData: "{}", ResponseData: "{}", SendSnapshot: snapshot.JSON()}); err != nil {
			t.Fatal(err)
		}
	}
	// Changing task parameters and deleting templates must never affect saved bodies.
	if err := db.Exec(`UPDATE push_tasks SET template_params = '{"code":"changed"}'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`DROP TABLE provider_templates`).Error; err != nil {
		t.Fatal(err)
	}
	if err := logs.UpdateStatus(2, "success", ""); err != nil {
		t.Fatal(err)
	}
	service := &AdminTaskService{pushTaskDAO: dao.NewPushTaskDAOWithDB(db), pushLogDAO: logs, messageDetails: s}
	detail, err := service.GetPushTask(1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.MessageDetail.LogID != 2 || detail.MessageDetail.Content != "第二次086697" || detail.MessageDetail.ProviderAccountID != 8 || detail.MessageDetail.SignatureValue != "已映射签名" {
		t.Fatalf("callback or current config replaced snapshot: %+v", detail.MessageDetail)
	}
	all, err := logs.GetByTaskID(task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	perLog := s.ForLogs(task, all)
	if perLog[1].Content != "第一次086697" || perLog[2].Content != "第二次086697" || perLog[3].Source != dto.MessageDetailUnavailable {
		t.Fatalf("attempt history changed: %+v", perLog)
	}
	response, err := service.GetPushTaskList(&dto.PushTaskListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(response)
	if strings.Contains(string(encoded), "message_detail") || strings.Contains(string(encoded), "第二次") {
		t.Fatalf("list loaded content details: %s", encoded)
	}
	// Even an orphaned log retains its snapshot.
	if s.ForLogs(nil, all)[2].Content != "第二次086697" {
		t.Fatal("snapshot depends on task existence")
	}
}

func TestMessageDetailUnavailableAndLegacyRequestKeys(t *testing.T) {
	s, _, task := newMessageDetailTestService(t)
	if s.Latest(task, nil).Source != dto.MessageDetailUnavailable {
		t.Fatal("pending task has content")
	}
	for _, raw := range []string{`{bad`, `null`, `{"version":99}`, `{"version":1,"unavailable_reason":"签名不存在"}`} {
		log := &model.PushLog{ID: 1, SendSnapshot: &raw}
		if got := s.Latest(task, []*model.PushLog{log}); got.Source != dto.MessageDetailUnavailable {
			t.Fatalf("bad/preparation snapshot was rendered: %+v", got)
		}
	}
	for _, key := range []string{"template_code", "template_id", "templateid"} {
		for _, value := range []string{`"16021"`, `16021`} {
			code, valid := legacyTemplateCode(`{"` + key + `":` + value + `}`)
			if !valid || code != "16021" {
				t.Fatalf("legacy key %s=%s resolved %q %v", key, value, code, valid)
			}
		}
	}
	for _, raw := range []string{`bad`, `[]`, `{"templateid":{}}`} {
		if _, valid := legacyTemplateCode(raw); valid {
			t.Fatalf("accepted malformed request %s", raw)
		}
	}
	task.TemplateParams = `{"code":123}`
	got := s.Latest(task, []*model.PushLog{{ID: 1, ProviderAccountID: 7, RequestData: `{"templateid":"16021"}`}})
	if got.Source != dto.MessageDetailUnavailable || !strings.Contains(got.UnavailableReason, "模板参数") {
		t.Fatalf("invalid parameter preview: %+v", got)
	}
}
