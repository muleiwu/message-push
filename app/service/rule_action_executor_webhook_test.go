package service

import (
	"context"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

func TestActionExecutorRetryAndSwitchDoNotCreateTerminalOutbox(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		actionConfig string
		wantPush     int
		wantDelayed  int
	}{
		{
			name:         "retry",
			action:       model.RuleActionRetry,
			actionConfig: `{"max_retry":2,"delay_seconds":1,"backoff_rate":2,"max_delay":10}`,
			wantDelayed:  1,
		},
		{
			name:         "switch provider",
			action:       model.RuleActionSwitchProvider,
			actionConfig: `{"exclude_current":true,"max_retry":2}`,
			wantPush:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTerminalServiceTestDB(t, true)
			task := createTerminalTestTask(t, db, "non-terminal-"+tt.action, "non-terminal-app")
			if err := db.Create(&model.Application{
				AppID:      task.AppID,
				AppName:    "non-terminal",
				AppSecret:  "secret",
				WebhookURL: "https://application.example/webhook",
			}).Error; err != nil {
				t.Fatalf("create application: %v", err)
			}

			producer := &actionExecutorProducerStub{}
			executor := newActionExecutorForTest(db, producer)
			result := executor.Execute(context.Background(), &ruleengine.EvaluateResult{
				Action: tt.action,
				MatchedRule: &model.FailureRule{
					Name:         tt.name,
					Action:       tt.action,
					ActionConfig: tt.actionConfig,
				},
			}, &ExecuteContext{
				Task:              task,
				ProviderAccountID: 7,
				ErrorCode:         "TEMPORARY",
				ErrorMessage:      "temporary provider failure",
			})

			if !result.ShouldRetry {
				t.Fatalf("execute result = %+v, want retry", result)
			}
			if producer.pushes != tt.wantPush || producer.delayed != tt.wantDelayed {
				t.Fatalf("producer calls = push %d delayed %d, want %d/%d",
					producer.pushes, producer.delayed, tt.wantPush, tt.wantDelayed)
			}
			var outboxCount int64
			if err := db.Model(&model.WebhookLog{}).Count(&outboxCount).Error; err != nil {
				t.Fatalf("count outbox: %v", err)
			}
			if outboxCount != 0 {
				t.Fatalf("terminal outbox count = %d, want 0", outboxCount)
			}
			var saved model.PushTask
			if err := db.Where("task_id = ?", task.TaskID).First(&saved).Error; err != nil {
				t.Fatalf("load task: %v", err)
			}
			if saved.Status != constants.TaskStatusPending {
				t.Fatalf("task status = %q, want pending", saved.Status)
			}
		})
	}
}

func TestActionExecutorFinalFailureCreatesRejectedOutbox(t *testing.T) {
	db := newTerminalServiceTestDB(t, true)
	task := createTerminalTestTask(t, db, "final-rejected", "final-app")
	if err := db.Create(&model.Application{
		AppID:      task.AppID,
		AppName:    "final",
		AppSecret:  "secret",
		WebhookURL: "https://application.example/webhook",
	}).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}

	executor := newActionExecutorForTest(db, &actionExecutorProducerStub{})
	result := executor.Execute(context.Background(), &ruleengine.EvaluateResult{
		Action: model.RuleActionFail,
	}, &ExecuteContext{
		Task:          task,
		ErrorCode:     "REJECTED",
		ErrorMessage:  "provider rejected message",
		TerminalEvent: constants.WebhookEventRejected,
	})
	if result.Err != nil || result.ShouldRetry || !result.TaskUpdated {
		t.Fatalf("execute result = %+v, want terminal failure", result)
	}

	var outbox model.WebhookLog
	if err := db.Where("task_id = ?", task.TaskID).First(&outbox).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if outbox.Event != constants.WebhookEventRejected ||
		outbox.Status != constants.WebhookDeliveryPending {
		t.Fatalf("outbox = %+v, want pending rejected event", outbox)
	}
}

func newActionExecutorForTest(db *gorm.DB, producer *actionExecutorProducerStub) *ActionExecutor {
	return &ActionExecutor{
		logger:          actionExecutorNoopLogger{},
		taskDAO:         dao.NewPushTaskDAOWithDB(db),
		logDAO:          dao.NewPushLogDAOWithDB(db),
		producer:        producer,
		terminalService: NewTaskTerminalServiceWithDB(db),
	}
}

type actionExecutorProducerStub struct {
	pushes  int
	delayed int
}

func (p *actionExecutorProducerStub) Push(context.Context, *model.PushTask) error {
	p.pushes++
	return nil
}

func (p *actionExecutorProducerStub) PushDelayed(context.Context, *model.PushTask, time.Time) error {
	p.delayed++
	return nil
}

func (p *actionExecutorProducerStub) PushBatch(context.Context, []*model.PushTask) error {
	return nil
}

type actionExecutorNoopLogger struct{}

func (actionExecutorNoopLogger) Debug(string, ...gsr.LoggerField)  {}
func (actionExecutorNoopLogger) Info(string, ...gsr.LoggerField)   {}
func (actionExecutorNoopLogger) Notice(string, ...gsr.LoggerField) {}
func (actionExecutorNoopLogger) Error(string, ...gsr.LoggerField)  {}
func (actionExecutorNoopLogger) Warn(string, ...gsr.LoggerField)   {}
func (actionExecutorNoopLogger) Fatal(string, ...gsr.LoggerField)  {}
