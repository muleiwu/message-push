package infrastructure

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	"cnb.cool/mliev/push/message-push/modules/sender"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCallbackTerminalEventsCreateOneOutbox(t *testing.T) {
	tests := []struct {
		callbackStatus string
		wantTaskStatus string
		wantEvent      string
		wantLogStatus  string
	}{
		{
			callbackStatus: constants.CallbackStatusDelivered,
			wantTaskStatus: constants.TaskStatusSuccess,
			wantEvent:      constants.WebhookEventDelivered,
			wantLogStatus:  constants.TaskStatusSuccess,
		},
		{
			callbackStatus: constants.CallbackStatusFailed,
			wantTaskStatus: constants.TaskStatusFailed,
			wantEvent:      constants.WebhookEventFailed,
			wantLogStatus:  constants.TaskStatusFailed,
		},
		{
			callbackStatus: constants.CallbackStatusRejected,
			wantTaskStatus: constants.TaskStatusFailed,
			wantEvent:      constants.WebhookEventRejected,
			wantLogStatus:  constants.TaskStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.callbackStatus, func(t *testing.T) {
			db := newCallbackServiceTestDB(t)
			task := &model.PushTask{
				TaskID:      "callback-" + tt.callbackStatus,
				AppID:       "callback-app",
				ChannelID:   1,
				MessageType: constants.MessageTypeSMS,
				Receiver:    "13800138000",
				Status:      constants.TaskStatusSent,
			}
			if err := db.Create(&model.Application{
				AppID:      task.AppID,
				AppName:    "callback",
				AppSecret:  "secret",
				WebhookURL: "https://application.example/webhook",
			}).Error; err != nil {
				t.Fatalf("create application: %v", err)
			}
			if err := db.Create(task).Error; err != nil {
				t.Fatalf("create task: %v", err)
			}
			pushLog := &model.PushLog{
				TaskID:            task.TaskID,
				AppID:             task.AppID,
				ProviderAccountID: 7,
				ProviderMsgID:     "provider-" + tt.callbackStatus,
				Status:            constants.TaskStatusSent,
			}
			if err := db.Create(pushLog).Error; err != nil {
				t.Fatalf("create push log: %v", err)
			}

			terminalService := service.NewTaskTerminalServiceWithDB(db)
			callbackService := &CallbackService{
				logger:          dispatcherNoopLogger{},
				taskDao:         dao.NewPushTaskDAOWithDB(db),
				logDao:          dao.NewPushLogDAOWithDB(db),
				callbackLogDao:  dao.NewCallbackLogDAOWithDB(db),
				ruleEngine:      callbackRuleEngineStub{},
				actionExecutor:  &callbackActionExecutorStub{terminalService: terminalService},
				terminalService: terminalService,
			}
			result := &sender.CallbackResult{
				ProviderID:   pushLog.ProviderMsgID,
				Status:       tt.callbackStatus,
				ErrorCode:    "CALLBACK_ERROR",
				ErrorMessage: "callback terminal failure",
				ReportTime:   time.Date(2026, 7, 30, 15, 0, 0, 0, time.UTC),
			}

			if err := callbackService.processCallbackResult(context.Background(), "provider", result, `{}`); err != nil {
				t.Fatalf("process callback: %v", err)
			}
			if err := callbackService.processCallbackResult(context.Background(), "provider", result, `{}`); err != nil {
				t.Fatalf("process duplicate callback: %v", err)
			}

			var savedTask model.PushTask
			if err := db.Where("task_id = ?", task.TaskID).First(&savedTask).Error; err != nil {
				t.Fatalf("load task: %v", err)
			}
			if savedTask.Status != tt.wantTaskStatus || savedTask.CallbackStatus != tt.callbackStatus {
				t.Fatalf("task status/callback = %q/%q, want %q/%q",
					savedTask.Status, savedTask.CallbackStatus, tt.wantTaskStatus, tt.callbackStatus)
			}
			var savedLog model.PushLog
			if err := db.First(&savedLog, pushLog.ID).Error; err != nil {
				t.Fatalf("load push log: %v", err)
			}
			if savedLog.Status != tt.wantLogStatus {
				t.Fatalf("push log status = %q, want %q", savedLog.Status, tt.wantLogStatus)
			}
			var outboxes []model.WebhookLog
			if err := db.Where("task_id = ?", task.TaskID).Find(&outboxes).Error; err != nil {
				t.Fatalf("load outbox: %v", err)
			}
			if len(outboxes) != 1 || outboxes[0].Event != tt.wantEvent {
				t.Fatalf("outboxes = %+v, want one %s event", outboxes, tt.wantEvent)
			}
		})
	}
}

type callbackRuleEngineStub struct{}

func (callbackRuleEngineStub) Evaluate(context.Context, *ruleengine.EvaluateRequest) *ruleengine.EvaluateResult {
	return &ruleengine.EvaluateResult{Action: model.RuleActionFail}
}

func (callbackRuleEngineStub) RefreshCache() {}

type callbackActionExecutorStub struct {
	terminalService *service.TaskTerminalService
}

func (s *callbackActionExecutorStub) Execute(
	ctx context.Context,
	_ *ruleengine.EvaluateResult,
	execCtx *service.ExecuteContext,
) *service.ExecuteResult {
	result, err := s.terminalService.Transition(ctx, service.TerminalTransition{
		TaskID:         execCtx.Task.TaskID,
		Status:         constants.TaskStatusFailed,
		Event:          execCtx.TerminalEvent,
		ErrorCode:      execCtx.ErrorCode,
		ErrorMessage:   execCtx.ErrorMessage,
		ProviderID:     execCtx.ProviderID,
		OccurredAt:     execCtx.OccurredAt,
		CallbackStatus: execCtx.CallbackStatus,
		CallbackTime:   execCtx.CallbackTime,
	})
	if err != nil {
		return &service.ExecuteResult{Action: model.RuleActionFail, Err: err}
	}
	if result.Changed {
		execCtx.Task.Status = constants.TaskStatusFailed
	}
	return &service.ExecuteResult{
		Action:      model.RuleActionFail,
		TaskUpdated: result.Changed,
	}
}

func newCallbackServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "callback.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE applications (
			id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, app_secret TEXT NOT NULL,
			app_name TEXT NOT NULL, status INTEGER, ip_whitelist TEXT, webhook_url TEXT,
			daily_quota INTEGER, rate_limit INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE push_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE, app_id TEXT NOT NULL,
			channel_id INTEGER NOT NULL, provider_account_id INTEGER, message_type TEXT NOT NULL, receiver TEXT NOT NULL,
			template_code TEXT, template_params TEXT, signature TEXT, status TEXT,
			callback_status TEXT, callback_time DATETIME, retry_count INTEGER, max_retry INTEGER,
			exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE push_logs (
			send_snapshot TEXT,
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL,
			provider_account_id INTEGER NOT NULL, provider_msg_id TEXT, request_data TEXT,
			response_data TEXT, status TEXT NOT NULL, error_message TEXT, cost_time INTEGER,
			created_at DATETIME
		)`,
		`CREATE TABLE callback_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, task_id TEXT, app_id TEXT NOT NULL,
			provider_code TEXT NOT NULL, provider_id TEXT, mobile TEXT, content TEXT,
			callback_status TEXT, error_code TEXT, error_message TEXT, raw_data TEXT, created_at DATETIME
		)`,
		`CREATE TABLE webhook_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, webhook_url TEXT NOT NULL,
			secret TEXT, events TEXT, status INTEGER, retry_count INTEGER, timeout INTEGER,
			description TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE webhook_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT, app_id TEXT NOT NULL,
			webhook_config_id INTEGER, webhook_url TEXT NOT NULL, event TEXT NOT NULL,
			request_data TEXT, response_status INTEGER, response_data TEXT, status TEXT NOT NULL,
			error_message TEXT, retry_count INTEGER, dedup_key TEXT UNIQUE, signing_secret TEXT,
			max_retries INTEGER, timeout_seconds INTEGER, next_attempt_at DATETIME,
			locked_until DATETIME, lease_token TEXT, created_at DATETIME, updated_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}
