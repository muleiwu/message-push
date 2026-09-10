package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"sync"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/migrations"
	"cnb.cool/mliev/push/message-push/modules/delivery"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	senderdomain "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type smsReporterFixture struct {
	mu       sync.Mutex
	calls    int
	response *senderdomain.SMSEventResponse
	err      error
	hook     func(context.Context, *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error)
}

func (r *smsReporterFixture) call(ctx context.Context, req *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error) {
	r.mu.Lock()
	r.calls++
	hook, response, err := r.hook, r.response, r.err
	r.mu.Unlock()
	if hook != nil {
		return hook(ctx, req)
	}
	return response, err
}
func (r *smsReporterFixture) QuerySMSEvents(ctx context.Context, req *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error) {
	return r.call(ctx, req)
}
func (r *smsReporterFixture) PullSMSEvents(ctx context.Context, req *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error) {
	return r.call(ctx, req)
}
func (r *smsReporterFixture) count() int { r.mu.Lock(); defer r.mu.Unlock(); return r.calls }

type smsFixture struct {
	s        *SMSEventService
	db       *gorm.DB
	now      time.Time
	reporter *smsReporterFixture
}

func newSMSFixture(t *testing.T) *smsFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/sms.db"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlFS, err := fs.Sub(migrations.FS(), "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, sqlFS, goose.WithDisableGlobalRegistry(true), goose.WithLogger(log.New(io.Discard, "", 0)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	f := &smsFixture{db: db, now: now, reporter: &smsReporterFixture{response: &senderdomain.SMSEventResponse{RequestID: "fixture-request", Items: []senderdomain.SMSEvent{{ProviderMsgID: "sid-1", Mobile: "+8613800138000", Status: "delivered", OccurredAt: now, RawData: `{"SerialNo":"sid-1"}`}}}}}
	f.s = NewSMSEventServiceWithDB(db)
	f.s.now = func() time.Time { return f.now }
	f.s.reporter = func(*model.ProviderAccount) (senderdomain.SMSReporter, error) { return f.reporter, nil }
	f.account(t, 1, "1400000001")
	return f
}

func (f *smsFixture) account(t *testing.T, id uint, appID string) {
	t.Helper()
	config, _ := json.Marshal(map[string]string{"secret_id": "fixture-id", "secret_key": "fixture-key", "sdk_app_id": appID})
	if err := f.db.Create(&model.ProviderAccount{ID: id, AccountCode: fmt.Sprintf("tc-%d", id), AccountName: "腾讯云测试", ProviderCode: constants.ProviderTencentSMS, ProviderType: "sms", Status: 1, Config: string(config)}).Error; err != nil {
		t.Fatal(err)
	}
}

func (f *smsFixture) send(t *testing.T, accountID uint, app, sid, signature string, at time.Time) *model.PushTask {
	t.Helper()
	var count int64
	f.db.Model(&model.Application{}).Where("app_id = ?", app).Count(&count)
	if count == 0 {
		if err := f.db.Create(&model.Application{AppID: app, AppName: app, AppSecret: "fixture", WebhookURL: "https://application.example/webhook"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	task := &model.PushTask{TaskID: app + "-" + sid, AppID: app, MessageType: "sms", Receiver: "13800138000", Status: constants.TaskStatusSent, TemplateParams: "{}", MaxRetry: 3, CreatedAt: at, UpdatedAt: at}
	if err := f.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	snapshot := (&model.SendSnapshot{Version: 1, SignatureValue: signature, ProviderAccountID: accountID, ProviderCode: constants.ProviderTencentSMS}).JSON()
	if err := f.db.Create(&model.PushLog{TaskID: task.TaskID, AppID: app, ProviderAccountID: accountID, ProviderMsgID: sid, Status: "sent", RequestData: "{}", ResponseData: "{}", SendSnapshot: snapshot, CreatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func smsCount(t *testing.T, db *gorm.DB, value any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(value).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSMSCollectionPersistsBeforeEffectsAndDeduplicatesSources(t *testing.T) {
	for _, callbackFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(callbackFirst), func(t *testing.T) {
			f := newSMSFixture(t)
			task := f.send(t, 1, "app-1", "sid-1", "测试", f.now.Add(-time.Minute))
			if callbackFirst {
				if _, err := f.s.IngestCallback(context.Background(), 1, f.reporter.response.Items); err != nil {
					t.Fatal(err)
				}
			}
			result, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if smsCount(t, f.db, &model.ProviderSMSEvent{}) != 1 || smsCount(t, f.db, &model.WebhookLog{}) != 0 {
				t.Fatal("ingestion executed business effects")
			}
			var before model.PushTask
			f.db.First(&before, task.ID)
			if before.Status != constants.TaskStatusSent {
				t.Fatal("task changed during ingestion")
			}
			if err = f.s.ProcessEvent(context.Background(), result.Items[0].ID); err != nil {
				t.Fatal(err)
			}
			if _, err = f.s.IngestCallback(context.Background(), 1, f.reporter.response.Items); err != nil {
				t.Fatal(err)
			}
			if _, err = f.s.Collect(context.Background(), 1, "reports", "query", SMSQueryRequest{PhoneNumber: "13800138000", BeginTime: f.now.Add(-time.Hour).Unix()}); err != nil {
				t.Fatal(err)
			}
			if err = f.s.ProcessPending(context.Background()); err != nil {
				t.Fatal(err)
			}
			var saved model.PushTask
			f.db.First(&saved, task.ID)
			if saved.Status != constants.TaskStatusSuccess || smsCount(t, f.db, &model.ProviderSMSEvent{}) != 1 || smsCount(t, f.db, &model.CallbackLog{}) != 1 || smsCount(t, f.db, &model.WebhookLog{}) != 1 {
				t.Fatalf("dedup/transition failed: task=%+v", saved)
			}
			var callback model.CallbackLog
			f.db.First(&callback)
			if callback.ProviderAccountID != 1 || callback.Attribution != "matched" {
				t.Fatalf("account attribution: %+v", callback)
			}
		})
	}
}

func TestSMSRepliesOnlyNotifyUniquelyMatchedApplications(t *testing.T) {
	for _, scenario := range []string{"unique", "ambiguous", "other account", "other signature", "future send", "missing signature"} {
		t.Run(scenario, func(t *testing.T) {
			f := newSMSFixture(t)
			f.account(t, 2, "1400000002")
			f.send(t, 2, "outside", "sid-outside", "测试", f.now.Add(-time.Minute))
			if scenario != "other account" {
				signature, at := "测试", f.now.Add(-time.Minute)
				if scenario == "other signature" {
					signature = "其他签名"
				}
				if scenario == "future send" {
					at = f.now.Add(time.Minute)
				}
				f.send(t, 1, "inside", "sid-inside", signature, at)
			}
			if scenario == "ambiguous" {
				f.send(t, 1, "inside-two", "sid-two", "测试", f.now.Add(-2*time.Minute))
			}
			signName := "测试"
			if scenario == "missing signature" {
				signName = ""
			}
			f.reporter.response = &senderdomain.SMSEventResponse{Items: []senderdomain.SMSEvent{{Mobile: "+8613800138000", Content: "STOP", SignName: signName, ExtendCode: "01", OccurredAt: f.now}}}
			result, err := f.s.Collect(context.Background(), 1, "replies", "query", SMSQueryRequest{PhoneNumber: "13800138000", BeginTime: f.now.Add(-time.Hour).Unix()})
			if err != nil {
				t.Fatal(err)
			}
			if err = f.s.ProcessEvent(context.Background(), result.Items[0].ID); err != nil {
				t.Fatal(err)
			}
			if _, err = f.s.Collect(context.Background(), 1, "replies", "pull", SMSQueryRequest{}); err != nil {
				t.Fatal(err)
			}
			if err = f.s.ProcessPending(context.Background()); err != nil {
				t.Fatal(err)
			}
			var event model.ProviderSMSEvent
			f.db.First(&event)
			want := "unmatched"
			if scenario == "unique" {
				want = "matched"
			}
			if scenario == "ambiguous" {
				want = "ambiguous"
			}
			if event.Attribution != want {
				t.Fatalf("attribution=%s, want %s", event.Attribution, want)
			}
			outboxes := int64(0)
			if scenario == "unique" {
				outboxes = 1
				if event.AppID != "inside" {
					t.Fatal("cross-account association")
				}
			}
			if smsCount(t, f.db, &model.CallbackLog{}) != 1 || smsCount(t, f.db, &model.WebhookLog{}) != outboxes {
				t.Fatal("duplicate or ambiguous notification")
			}
		})
	}
}

type smsRuleFixture struct {
	ruleengine.Engine
	action string
}

func (s smsRuleFixture) Evaluate(context.Context, *ruleengine.EvaluateRequest) *ruleengine.EvaluateResult {
	return &ruleengine.EvaluateResult{Action: s.action, MatchedRule: &model.FailureRule{Action: s.action, ActionConfig: `{"max_retry":3,"delay_seconds":1,"backoff_rate":2,"max_delay":10,"exclude_current":true}`}}
}

type smsProducerFixture struct {
	delivery.Producer
	keys               map[string]bool
	attempts, accepted int
	uncertainOnce      bool
}

func (p *smsProducerFixture) PushDelayedOnce(_ context.Context, _ *model.PushTask, _ time.Time, key string) error {
	p.attempts++
	if !p.keys[key] {
		p.keys[key] = true
		p.accepted++
	}
	if p.uncertainOnce {
		p.uncertainOnce = false
		return errors.New("lost Redis acknowledgement")
	}
	return nil
}

func TestSMSFailureDecisionAndQueueEffectSurviveReplay(t *testing.T) {
	for _, action := range []string{model.RuleActionRetry, model.RuleActionSwitchProvider} {
		t.Run(action, func(t *testing.T) {
			f := newSMSFixture(t)
			task := f.send(t, 1, "retry-app", "sid-1", "测试", f.now.Add(-time.Minute))
			f.reporter.response.Items[0].Status = "failed"
			producer := &smsProducerFixture{keys: map[string]bool{}, uncertainOnce: true}
			f.s.producer = producer
			f.s.engine = smsRuleFixture{action: action}
			result, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
			if err != nil {
				t.Fatal(err)
			}
			id := result.Items[0].ID
			if err = f.s.ProcessEvent(context.Background(), id); err == nil {
				t.Fatal("expected dispatch acknowledgement failure")
			}
			var saved model.PushTask
			f.db.First(&saved, task.ID)
			if saved.RetryCount != 1 || saved.Status != constants.TaskStatusPending {
				t.Fatalf("decision was not committed: %+v", saved)
			}
			restarted := NewSMSEventServiceWithDB(f.db)
			restarted.producer = producer
			restarted.now = f.s.now
			if err = restarted.RetryEvent(context.Background(), 1, id); err != nil {
				t.Fatal(err)
			}
			if err = restarted.ProcessEvent(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			f.db.First(&saved, task.ID)
			if saved.RetryCount != 1 || producer.accepted != 1 || producer.attempts != 2 {
				t.Fatalf("effect repeated: retry=%d producer=%+v", saved.RetryCount, producer)
			}
			if smsCount(t, f.db, &model.CallbackLog{}) != 1 || smsCount(t, f.db, &model.WebhookLog{}) != 0 {
				t.Fatal("retry emitted terminal notification")
			}
		})
	}
}

func TestSMSLocalTransactionFailureCanRetryWithoutCloudRead(t *testing.T) {
	f := newSMSFixture(t)
	f.send(t, 1, "rollback", "sid-1", "测试", f.now.Add(-time.Minute))
	result, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.Callback().Create().Before("gorm:create").Register("sms_outbox_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "webhook_logs" {
			tx.AddError(errors.New("outbox write failed"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err = f.s.ProcessEvent(context.Background(), result.Items[0].ID); err == nil {
		t.Fatal("expected local transaction failure")
	}
	if smsCount(t, f.db, &model.CallbackLog{}) != 0 {
		t.Fatal("callback log escaped rolled-back transaction")
	}
	f.db.Callback().Create().Remove("sms_outbox_failure")
	if err = f.s.RetryEvent(context.Background(), 1, result.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err = f.s.ProcessPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.reporter.count() != 1 || smsCount(t, f.db, &model.WebhookLog{}) != 1 {
		t.Fatal("retry consumed another batch or lost notification")
	}
}

func TestSMSConsumptionUncertaintyPausesAndRestartDoesNotRepeat(t *testing.T) {
	f := newSMSFixture(t)
	config, err := f.s.GetPolling(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if config.Streams[0].Enabled || config.Streams[1].Enabled || smsCount(t, f.db, &model.SMSPollingStream{}) != 0 {
		t.Fatal("read enabled or persisted default polling")
	}
	if _, err = f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	f.reporter.err = &senderdomain.RemoteResourceError{Code: "TRANSPORT_ERROR", Message: "response lost", Uncertain: true}
	if _, err = f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{}); err == nil {
		t.Fatal("expected uncertain pull")
	}
	if _, err = f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{}); !errors.Is(err, ErrSMSPaused) {
		t.Fatalf("uncertain consumption retried: %v", err)
	}
	if f.reporter.count() != 1 {
		t.Fatal("queue consumed twice")
	}
	var state model.SMSPollingStream
	f.db.Where("kind = ?", "reports").First(&state)
	if !state.Suspended || state.SuspendReason == "" {
		t.Fatal("uncertainty not visible")
	}
	// An interrupted run is persisted before any SDK call and recovered as uncertain.
	if err = f.db.Create(&model.SMSPullRun{ProviderAccountID: 1, SDKAppID: "1400000001", Kind: "replies", Source: "poll", State: "started", StartedAt: f.now.Add(-time.Minute - time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.s.RecoverInterruptedPulls(context.Background()); err != nil {
		t.Fatal(err)
	}
	var replyState model.SMSPollingStream
	f.db.Where("kind = ?", "replies").First(&replyState)
	if !replyState.Suspended {
		t.Fatal("restart would repeat uncertain queue consumption")
	}
	if _, err = f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30, ResumeReports: true}); err != nil {
		t.Fatal(err)
	}
	f.db.First(&state, state.ID)
	if state.Suspended {
		t.Fatal("explicit resume did not clear pause")
	}
}

func TestSMSPollingOwnershipBoundsAndDisabledAccounts(t *testing.T) {
	f := newSMSFixture(t)
	f.account(t, 2, "1400000001")
	if _, err := f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 9}); !errors.Is(err, ErrSMSInvalid) {
		t.Fatal("invalid interval accepted")
	}
	if _, err := f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.UpdatePolling(context.Background(), 2, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); !errors.Is(err, ErrSMSBusy) {
		t.Fatalf("duplicate queue consumer: %v", err)
	}
	f.reporter.response.LimitReached = true
	if err := f.s.PollDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.reporter.count() != 5 {
		t.Fatalf("poll round not bounded: %d", f.reporter.count())
	}
	if err := f.db.Model(&model.ProviderAccount{}).Where("id = ?", 1).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	if err := f.s.PollDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.reporter.count() != 5 {
		t.Fatal("disabled account still consumed queue")
	}
}

func TestSMSManualAndAutomaticConsumptionAreMutuallyExclusive(t *testing.T) {
	f := newSMSFixture(t)
	if _, err := f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	f.reporter.hook = func(ctx context.Context, _ *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error) {
		close(entered)
		select {
		case <-release:
			return f.reporter.response, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
		done <- err
	}()
	<-entered
	if err := f.s.PollDue(context.Background()); err != nil {
		close(release)
		t.Fatal(err)
	}
	_, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
	close(release)
	if !errors.Is(err, ErrSMSBusy) {
		t.Fatalf("concurrent manual pull accepted: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if f.reporter.count() != 1 {
		t.Fatal("concurrent poll consumed queue")
	}
}

func TestSMSInvalidOrCrossAccountFactsDoNotChangeTasks(t *testing.T) {
	f := newSMSFixture(t)
	f.account(t, 2, "1400000002")
	task := f.send(t, 2, "other", "sid-1", "测试", f.now.Add(-time.Minute))
	first, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.ProcessEvent(context.Background(), first.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	var saved model.PushTask
	f.db.First(&saved, task.ID)
	if saved.Status != constants.TaskStatusSent {
		t.Fatal("cross-account receipt changed task")
	}
	f.reporter.response.Items[0].Status = "unknown"
	second, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.ProcessEvent(context.Background(), second.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	var event model.ProviderSMSEvent
	f.db.First(&event, second.Items[0].ID)
	if event.State != "invalid" {
		t.Fatal("unknown status treated as failed delivery")
	}
	if err = f.s.RetryEvent(context.Background(), 2, event.ID); !errors.Is(err, ErrSMSNotFound) {
		t.Fatal("cross-account retry allowed")
	}
	if smsCount(t, f.db, &model.WebhookLog{}) != 0 {
		t.Fatal("unknown/cross-account event notified application")
	}
}

func TestSMSResponsePersistenceFailureKeepsLedgerAndStopsConsumption(t *testing.T) {
	f := newSMSFixture(t)
	f.reporter.hook = func(context.Context, *senderdomain.SMSEventRequest) (*senderdomain.SMSEventResponse, error) {
		var count int64
		if err := f.db.Model(&model.SMSPullRun{}).Where("state = ?", "started").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatal("cloud call started before its durable ledger")
		}
		return f.reporter.response, nil
	}
	if err := f.db.Callback().Create().Before("gorm:create").Register("sms_inbox_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "provider_sms_events" {
			tx.AddError(errors.New("inbox unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer f.db.Callback().Create().Remove("sms_inbox_failure")
	_, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{})
	var remote *senderdomain.RemoteResourceError
	if !errors.As(err, &remote) || !remote.Uncertain || remote.Code != "PERSISTENCE_FAILED" {
		t.Fatalf("persistence failure was hidden: %v", err)
	}
	var run model.SMSPullRun
	if err := f.db.First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.State != "uncertain" {
		t.Fatalf("lost execution ledger: %+v", run)
	}
	if _, err := f.s.Collect(context.Background(), 1, "reports", "pull", SMSQueryRequest{}); !errors.Is(err, ErrSMSPaused) {
		t.Fatalf("queue was consumed again: %v", err)
	}
	if f.reporter.count() != 1 {
		t.Fatal("unexpected second cloud call")
	}
}

func TestSMSDeletedAccountReleasesPollingOwnership(t *testing.T) {
	f := newSMSFixture(t)
	f.account(t, 2, "1400000001")
	if _, err := f.s.UpdatePolling(context.Background(), 1, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Delete(&model.ProviderAccount{}, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.PollDue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.reporter.count() != 0 {
		t.Fatal("deleted account consumed queue")
	}
	if _, err := f.s.UpdatePolling(context.Background(), 2, SMSPollingRequest{ReportsEnabled: true, IntervalSeconds: 30}); err != nil {
		t.Fatalf("deleted owner blocked replacement: %v", err)
	}
}
