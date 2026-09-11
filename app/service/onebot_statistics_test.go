package service

import (
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
)

func TestQQStatisticsFilterAndOnboardingHealth(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	createdAt := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
	for _, task := range []*model.PushTask{
		{TaskID: "qq-success", AppID: "qq-app", ChannelID: 7, MessageType: constants.MessageTypeQQ, Receiver: "private:123", Status: constants.TaskStatusSuccess, CreatedAt: createdAt},
		{TaskID: "qq-failed", AppID: "qq-app", ChannelID: 7, MessageType: constants.MessageTypeQQ, Receiver: "group:456", Status: constants.TaskStatusFailed, CreatedAt: createdAt},
		{TaskID: "sms-success", AppID: "qq-app", ChannelID: 8, MessageType: constants.MessageTypeSMS, Receiver: "13800138000", Status: constants.TaskStatusSuccess, CreatedAt: createdAt},
	} {
		if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &AdminStatisticsService{db: db}
	stats, err := s.GetStatistics(&dto.StatisticsRequest{StartDate: "2026-09-11", EndDate: "2026-09-11", MessageType: constants.MessageTypeQQ})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Summary.TotalCount != 2 || stats.Summary.SuccessCount != 1 || stats.Summary.FailureCount != 1 || len(stats.MessageTypeDistribution) != 1 || stats.MessageTypeDistribution[0].MessageType != constants.MessageTypeQQ {
		t.Fatalf("QQ filter returned wrong counts: %+v", stats)
	}
	for _, withChannel := range []bool{false, true} {
		var channels []*model.Channel
		wantTotal := 0
		if withChannel {
			channels = []*model.Channel{{ID: 7, Type: constants.MessageTypeQQ}}
			wantTotal = 1
		}
		health, _ := buildChannelTypeHealth(channels, map[uint]*dto.ChannelReadinessResponse{7: {State: constants.ChannelReadinessReady}})
		found := false
		for _, item := range health {
			if item.Type == constants.MessageTypeQQ {
				found = true
				if item.Total != wantTotal || item.Healthy != wantTotal {
					t.Fatalf("QQ health=%+v", item)
				}
			}
		}
		if !found {
			t.Fatal("QQ missing from onboarding types")
		}
	}
}
