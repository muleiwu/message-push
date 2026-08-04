package infrastructure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

func TestWebhookDispatcherDeliversSignedOutbox(t *testing.T) {
	var requestCount atomic.Int32
	var deliveryID, signature, timestamp string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		deliveryID = r.Header.Get("X-Webhook-Delivery-ID")
		signature = r.Header.Get("X-Webhook-Signature")
		timestamp = r.Header.Get("X-Webhook-Timestamp")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	db := newWebhookDispatcherTestDB(t)
	now := time.Date(2026, 7, 30, 21, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	outbox := createDispatcherTestOutbox(t, db, server.URL, now)
	dispatcher := NewWebhookDispatcherWithDB(db, dispatcherNoopLogger{})
	dispatcher.now = func() time.Time { return now }

	dispatcher.DispatchOnce(context.Background())
	dispatcher.wg.Wait()

	var saved model.WebhookLog
	if err := db.First(&saved, outbox.ID).Error; err != nil {
		t.Fatalf("load webhook log: %v", err)
	}
	if saved.Status != constants.WebhookDeliverySuccess || saved.ResponseStatus != http.StatusNoContent {
		t.Fatalf("delivery result = status %q response %d", saved.Status, saved.ResponseStatus)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("request count = %d, want 1", requestCount.Load())
	}
	if deliveryID != strconv.FormatUint(uint64(outbox.ID), 10) {
		t.Fatalf("delivery id header = %q", deliveryID)
	}
	attemptTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		t.Fatalf("parse timestamp header: %v", err)
	}
	wantSignature := generateWebhookSignature([]byte(outbox.RequestData), outbox.SigningSecret, attemptTimestamp)
	if signature != wantSignature {
		t.Fatalf("signature = %q, want %q", signature, wantSignature)
	}
}

func TestWebhookDispatcherRetriesThenFails(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		http.Error(w, "temporary failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	db := newWebhookDispatcherTestDB(t)
	now := time.Date(2026, 7, 30, 13, 30, 0, 0, time.UTC)
	outbox := createDispatcherTestOutbox(t, db, server.URL, now)
	if err := db.Model(outbox).Update("max_retries", 1).Error; err != nil {
		t.Fatalf("set retry count: %v", err)
	}

	dispatcher := NewWebhookDispatcherWithDB(db, dispatcherNoopLogger{})
	dispatcher.now = func() time.Time { return now }
	dispatcher.DispatchOnce(context.Background())
	dispatcher.wg.Wait()

	var afterFirst model.WebhookLog
	if err := db.First(&afterFirst, outbox.ID).Error; err != nil {
		t.Fatalf("load first attempt: %v", err)
	}
	if afterFirst.Status != constants.WebhookDeliveryPending || afterFirst.RetryCount != 1 {
		t.Fatalf("first attempt = status %q retries %d", afterFirst.Status, afterFirst.RetryCount)
	}
	if afterFirst.NextAttemptAt == nil || !afterFirst.NextAttemptAt.Equal(now.Add(time.Second)) {
		t.Fatalf("next attempt = %v, want %v", afterFirst.NextAttemptAt, now.Add(time.Second))
	}

	now = now.Add(time.Second)
	dispatcher.DispatchOnce(context.Background())
	dispatcher.wg.Wait()

	var exhausted model.WebhookLog
	if err := db.First(&exhausted, outbox.ID).Error; err != nil {
		t.Fatalf("load exhausted delivery: %v", err)
	}
	if exhausted.Status != constants.WebhookDeliveryFailed || exhausted.RetryCount != 1 {
		t.Fatalf("exhausted delivery = status %q retries %d", exhausted.Status, exhausted.RetryCount)
	}
	if requestCount.Load() != 2 {
		t.Fatalf("request count = %d, want 2", requestCount.Load())
	}
}

func TestWebhookDispatcherReclaimsExpiredLeaseOnlyOnce(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := newWebhookDispatcherTestDB(t)
	now := time.Date(2026, 7, 30, 14, 0, 0, 0, time.UTC)
	outbox := createDispatcherTestOutbox(t, db, server.URL, now.Add(-time.Minute))
	expired := now.Add(-time.Second)
	if err := db.Model(outbox).Updates(map[string]interface{}{
		"status":       constants.WebhookDeliveryProcessing,
		"lease_token":  "dead-worker",
		"locked_until": expired,
	}).Error; err != nil {
		t.Fatalf("expire lease: %v", err)
	}

	firstDispatcher := NewWebhookDispatcherWithDB(db, dispatcherNoopLogger{})
	firstDispatcher.now = func() time.Time { return now }
	secondDispatcher := NewWebhookDispatcherWithDB(db, dispatcherNoopLogger{})
	secondDispatcher.now = func() time.Time { return now }
	firstDispatcher.DispatchOnce(context.Background())
	secondDispatcher.DispatchOnce(context.Background())
	firstDispatcher.wg.Wait()
	secondDispatcher.wg.Wait()

	if requestCount.Load() != 1 {
		t.Fatalf("request count = %d, want 1", requestCount.Load())
	}
}

func TestWebhookDispatcherRetriesNetworkErrors(t *testing.T) {
	db := newWebhookDispatcherTestDB(t)
	now := time.Date(2026, 7, 30, 14, 30, 0, 0, time.UTC)
	outbox := createDispatcherTestOutbox(t, db, "https://example.invalid/webhook", now)

	dispatcher := NewWebhookDispatcherWithDB(db, dispatcherNoopLogger{})
	dispatcher.now = func() time.Time { return now }
	dispatcher.httpDoer = func(time.Duration) webhookHTTPDoer {
		return webhookErrorDoer{}
	}
	dispatcher.DispatchOnce(context.Background())
	dispatcher.wg.Wait()

	var saved model.WebhookLog
	if err := db.First(&saved, outbox.ID).Error; err != nil {
		t.Fatalf("load webhook log: %v", err)
	}
	if saved.Status != constants.WebhookDeliveryPending || saved.RetryCount != 1 {
		t.Fatalf("network error result = status %q retry %d", saved.Status, saved.RetryCount)
	}
	if saved.ErrorMessage == "" || saved.NextAttemptAt == nil ||
		!saved.NextAttemptAt.Equal(now.Add(time.Second)) {
		t.Fatalf("network error metadata = error %q next %v", saved.ErrorMessage, saved.NextAttemptAt)
	}
}

func newWebhookDispatcherTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "dispatcher.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE webhook_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT, app_id TEXT NOT NULL,
		webhook_config_id INTEGER, webhook_url TEXT NOT NULL, event TEXT NOT NULL,
		request_data TEXT, response_status INTEGER, response_data TEXT, status TEXT NOT NULL,
		error_message TEXT, retry_count INTEGER, dedup_key TEXT UNIQUE, signing_secret TEXT,
		max_retries INTEGER, timeout_seconds INTEGER, next_attempt_at DATETIME,
		locked_until DATETIME, lease_token TEXT, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create webhook_logs: %v", err)
	}
	return db
}

func createDispatcherTestOutbox(
	t *testing.T,
	db *gorm.DB,
	webhookURL string,
	now time.Time,
) *model.WebhookLog {
	t.Helper()
	outbox := &model.WebhookLog{
		TaskID:         "dispatcher-task",
		AppID:          "dispatcher-app",
		WebhookURL:     webhookURL,
		Event:          constants.WebhookEventFailed,
		RequestData:    `{"event":"failed","task_id":"dispatcher-task"}`,
		Status:         constants.WebhookDeliveryPending,
		DedupKey:       fmt.Sprintf("dispatcher:%d", now.UnixNano()),
		SigningSecret:  "dispatcher-secret",
		MaxRetries:     3,
		TimeoutSeconds: 5,
		NextAttemptAt:  &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := dao.NewWebhookLogDAOWithDB(db).Create(outbox); err != nil {
		t.Fatalf("create outbox: %v", err)
	}
	return outbox
}

type dispatcherNoopLogger struct{}

func (dispatcherNoopLogger) Debug(string, ...gsr.LoggerField)  {}
func (dispatcherNoopLogger) Info(string, ...gsr.LoggerField)   {}
func (dispatcherNoopLogger) Notice(string, ...gsr.LoggerField) {}
func (dispatcherNoopLogger) Error(string, ...gsr.LoggerField)  {}
func (dispatcherNoopLogger) Warn(string, ...gsr.LoggerField)   {}
func (dispatcherNoopLogger) Fatal(string, ...gsr.LoggerField)  {}

type webhookErrorDoer struct{}

func (webhookErrorDoer) Do(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network unavailable")
}
