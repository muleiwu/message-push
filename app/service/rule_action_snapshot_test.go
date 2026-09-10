package service

import (
	"context"
	"encoding/json"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
)

func TestRuleActionsPreserveSendSnapshotAndLeaveCallbackActionsEmpty(t *testing.T) {
	for _, action := range []string{model.RuleActionRetry, model.RuleActionSwitchProvider, model.RuleActionFail, model.RuleActionAlert} {
		for _, fromCallback := range []bool{false, true} {
			t.Run(action+map[bool]string{true: "/callback", false: "/send"}[fromCallback], func(t *testing.T) {
				db := newTerminalServiceTestDB(t, true)
				task := createTerminalTestTask(t, db, "action-snapshot", "app")
				executor := newActionExecutorForTest(db, &actionExecutorProducerStub{})
				ctx := &ExecuteContext{Task: task, ProviderAccountID: 7, RequestData: "{}", ErrorMessage: "provider error"}
				if !fromCallback {
					ctx.SendSnapshot = &model.SendSnapshot{Version: 1, ProviderAccountID: 7, MessageContent: model.MessageContent{Content: "发送时正文", MappedParams: map[string]string{"code": "086697"}}}
				}
				result := executor.Execute(context.Background(), &ruleengine.EvaluateResult{Action: action, MatchedRule: &model.FailureRule{Action: action, ActionConfig: `{"max_retry":3,"delay_seconds":1}`}}, ctx)
				if result.Err != nil {
					t.Fatal(result.Err)
				}
				logs, err := executor.logDAO.GetByTaskID(task.TaskID)
				if err != nil || len(logs) != 1 {
					t.Fatalf("logs=%+v error=%v", logs, err)
				}
				if fromCallback {
					if logs[0].SendSnapshot != nil {
						t.Fatal("callback created a send snapshot")
					}
					return
				}
				if logs[0].SendSnapshot == nil {
					t.Fatal("send action lost snapshot")
				}
				var got model.SendSnapshot
				if err := json.Unmarshal([]byte(*logs[0].SendSnapshot), &got); err != nil {
					t.Fatal(err)
				}
				if got.Content != "发送时正文" || got.MappedParams["code"] != "086697" {
					t.Fatalf("action changed snapshot: %+v", got)
				}
			})
		}
	}
}
