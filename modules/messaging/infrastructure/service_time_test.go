package infrastructure

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestQueryTaskNormalizationProducesUTCJSON(t *testing.T) {
	shanghai := time.FixedZone("UTC+8", 8*60*60)
	callback := time.Date(2026, 8, 4, 18, 0, 0, 0, shanghai)
	scheduled := callback.Add(time.Hour)
	task := &model.PushTask{
		CallbackTime: &callback,
		ScheduledAt:  &scheduled,
		CreatedAt:    callback,
		UpdatedAt:    callback.Add(5 * time.Minute),
	}

	normalizePushTaskTimes(task)
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	if strings.Contains(string(payload), "+08:00") || !strings.Contains(string(payload), "2026-08-04T10:00:00Z") {
		t.Fatalf("task JSON is not UTC: %s", payload)
	}
}
