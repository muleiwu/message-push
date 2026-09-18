package infrastructure

import (
	"context"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/ruleengine/domain"
	"github.com/muleiwu/gsr"
)

type oneBotRuleLogger struct{ gsr.Logger }

func (oneBotRuleLogger) Info(string, ...gsr.LoggerField) {}

func TestOneBotAsyncOnlyRetriesWhenARuleMatches(t *testing.T) {
	for _, tt := range []struct {
		name, provider, code, ruleProvider, ruleCode, want string
		matched                                            bool
	}{
		{"async fallback", constants.ProviderOneBot, constants.ErrorCodeOneBotAsyncUnsupported, "smtp", "", model.RuleActionFail, false},
		{"explicit retry", constants.ProviderOneBot, constants.ErrorCodeOneBotAsyncUnsupported, constants.ProviderOneBot, constants.ErrorCodeOneBotAsyncUnsupported, model.RuleActionRetry, true},
		{"wildcard rule", constants.ProviderOneBot, constants.ErrorCodeOneBotAsyncUnsupported, "", "", model.RuleActionRetry, true},
		{"unrelated OneBot failure", constants.ProviderOneBot, "HTTP_500", "smtp", "", model.RuleActionRetry, false},
		{"other provider", "smtp", constants.ErrorCodeOneBotAsyncUnsupported, constants.ProviderOneBot, "", model.RuleActionRetry, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := &RuleEngineService{logger: oneBotRuleLogger{}, cache: &ruleCache{rules: map[string][]*model.FailureRule{
				model.RuleSceneSendFailure: {{ProviderCode: tt.ruleProvider, ErrorCode: tt.ruleCode, Action: model.RuleActionRetry}},
			}}}
			got := s.Evaluate(context.Background(), &domain.EvaluateRequest{Scene: model.RuleSceneSendFailure, ProviderCode: tt.provider, MessageType: constants.MessageTypeQQ, ErrorCode: tt.code})
			if got.Action != tt.want || got.HasMatch != tt.matched {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}
