package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"cnb.cool/mliev/push/message-push/modules/delivery"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/lock"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	"cnb.cool/mliev/push/message-push/modules/sender"
	senderdomain "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSMSInvalid  = errors.New("短信操作参数无效")
	ErrSMSBusy     = errors.New("该短信队列有操作正在进行，请稍后重试")
	ErrSMSPaused   = errors.New("队列拉取已暂停，请先核对执行记录并恢复")
	ErrSMSNotFound = errors.New("短信记录不存在")
	smsLocalLocks  sync.Map
)

type SMSQueryRequest struct {
	PhoneNumber string `json:"phone_number"`
	BeginTime   int64  `json:"begin_time"`
	EndTime     int64  `json:"end_time"`
	Limit       int    `json:"limit"`
}

type SMSCollectionResult struct {
	RunID        uint64                    `json:"run_id,omitempty"`
	RequestID    string                    `json:"request_id"`
	Received     int                       `json:"received"`
	Inserted     int                       `json:"inserted"`
	Duplicates   int                       `json:"duplicates"`
	Pending      int                       `json:"pending"`
	LimitReached bool                      `json:"limit_reached"`
	Items        []*model.ProviderSMSEvent `json:"items"`
}

type SMSPollingRequest struct {
	ReportsEnabled  bool `json:"reports_enabled"`
	RepliesEnabled  bool `json:"replies_enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
	ResumeReports   bool `json:"resume_reports"`
	ResumeReplies   bool `json:"resume_replies"`
}

type SMSPollingResponse struct {
	Streams []model.SMSPollingStream `json:"streams"`
	Runs    []model.SMSPullRun       `json:"runs"`
}

// SMSEventService owns the durable boundary between cloud reads/consumption
// and local business effects. It never loads credentials into persisted rows.
type SMSEventService struct {
	db              *gorm.DB
	now             func() time.Time
	acquire         func(context.Context, string) (func(), error)
	reporter        func(*model.ProviderAccount) (senderdomain.SMSReporter, error)
	producer        delivery.Producer
	engine          ruleengine.Engine
	logger          gsr.Logger
	defaultAlertURL string
}

func NewSMSEventService() *SMSEventService {
	s := NewSMSEventServiceWithDB(helper.GetDatabase())
	s.logger, s.producer, s.engine = helper.GetLogger(), delivery.GetProducer(), ruleengine.GetEngine()
	s.defaultAlertURL = helper.GetEnv().GetString("alert.default_webhook_url", "")
	s.acquire = func(ctx context.Context, scope string) (func(), error) {
		l := lock.NewRedisLock(helper.GetRedis(), "tencent-sms:"+scope, 60*time.Second)
		ok, err := l.TryLock(ctx)
		if err != nil {
			return nil, fmt.Errorf("无法获取短信操作锁: %w", err)
		}
		if !ok {
			return nil, ErrSMSBusy
		}
		return func() {
			c, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = l.Unlock(c)
		}, nil
	}
	return s
}

func NewSMSEventServiceWithDB(db *gorm.DB) *SMSEventService {
	return &SMSEventService{db: db, now: timeutil.Now, logger: smsQuietLogger{},
		reporter: func(account *model.ProviderAccount) (senderdomain.SMSReporter, error) {
			impl, err := sender.GetResolver().GetSender(account.ProviderCode)
			if err != nil {
				return nil, err
			}
			reporter, ok := impl.(senderdomain.SMSReporter)
			if !ok {
				return nil, senderdomain.ErrResourceUnsupported
			}
			return reporter, nil
		},
		acquire: func(ctx context.Context, key string) (func(), error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entry, _ := smsLocalLocks.LoadOrStore(key, make(chan struct{}, 1))
			ch := entry.(chan struct{})
			select {
			case ch <- struct{}{}:
				return func() { <-ch }, nil
			default:
				return nil, ErrSMSBusy
			}
		},
	}
}

type smsQuietLogger struct{}

func (smsQuietLogger) Debug(string, ...gsr.LoggerField)  {}
func (smsQuietLogger) Info(string, ...gsr.LoggerField)   {}
func (smsQuietLogger) Notice(string, ...gsr.LoggerField) {}
func (smsQuietLogger) Warn(string, ...gsr.LoggerField)   {}
func (smsQuietLogger) Error(string, ...gsr.LoggerField)  {}
func (smsQuietLogger) Fatal(string, ...gsr.LoggerField)  {}

func (s *SMSEventService) account(ctx context.Context, id uint, enabled bool) (*model.ProviderAccount, string, error) {
	var account model.ProviderAccount
	if err := s.db.WithContext(ctx).First(&account, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", ErrSMSNotFound
		}
		return nil, "", err
	}
	if account.ProviderCode != constants.ProviderTencentSMS || account.ProviderType != constants.MessageTypeSMS || (enabled && account.Status != 1) {
		return nil, "", fmt.Errorf("%w：请选择已启用的腾讯云短信账号", ErrSMSInvalid)
	}
	config, err := account.GetConfig()
	if err != nil {
		return nil, "", fmt.Errorf("%w：账号配置无效", ErrSMSInvalid)
	}
	appID, _ := config["sdk_app_id"].(string)
	if strings.TrimSpace(appID) == "" {
		return nil, "", fmt.Errorf("%w：请配置 SmsSdkAppId", ErrSMSInvalid)
	}
	if enabled {
		secretID, _ := config["secret_id"].(string)
		secretKey, _ := config["secret_key"].(string)
		if secretID == "" || secretKey == "" {
			return nil, "", fmt.Errorf("%w：请配置腾讯云 SecretId 和 SecretKey", ErrSMSInvalid)
		}
	}
	return &account, appID, nil
}

func smsKindValid(kind string) bool {
	return kind == senderdomain.SMSReports || kind == senderdomain.SMSReplies
}

func (s *SMSEventService) ensureStreams(ctx context.Context, accountID uint, appID string) error {
	for _, kind := range []string{senderdomain.SMSReports, senderdomain.SMSReplies} {
		row := model.SMSPollingStream{ProviderAccountID: accountID, SDKAppID: appID, Kind: kind, IntervalSeconds: 30, UpdatedAt: s.now()}
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider_account_id"}, {Name: "kind"}}, DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SMSEventService) Collect(ctx context.Context, accountID uint, kind, source string, req SMSQueryRequest) (*SMSCollectionResult, error) {
	if !smsKindValid(kind) || (source != "query" && source != "pull" && source != "poll") {
		return nil, ErrSMSInvalid
	}
	if req.Limit == 0 {
		req.Limit = 100
	}
	if req.Limit < 1 || req.Limit > 100 {
		return nil, fmt.Errorf("%w：条数须为 1～100", ErrSMSInvalid)
	}
	account, appID, err := s.account(ctx, accountID, true)
	if err != nil {
		return nil, err
	}
	request := &senderdomain.SMSEventRequest{Account: account, Kind: kind, Limit: req.Limit}
	if source == "query" {
		phone := apphelper.ParsePhoneNumber(req.PhoneNumber)
		now := s.now().Truncate(time.Second)
		if req.EndTime == 0 {
			req.EndTime = now.Unix()
		}
		if !phone.Valid || phone.CountryCode != "86" {
			return nil, fmt.Errorf("%w：请填写有效的中国大陆手机号码", ErrSMSInvalid)
		}
		if req.BeginTime < now.Add(-7*24*time.Hour).Unix() || req.BeginTime > req.EndTime || req.EndTime > now.Unix() {
			return nil, fmt.Errorf("%w：查询范围须在最近七天内", ErrSMSInvalid)
		}
		request.PhoneNumber, request.BeginTime, request.EndTime = phone.E164, time.Unix(req.BeginTime, 0), time.Unix(req.EndTime, 0)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	unlock, err := s.acquire(ctx, appID+":"+kind)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.ensureStreams(ctx, accountID, appID); err != nil {
		return nil, err
	}
	var stream model.SMSPollingStream
	if err := s.db.WithContext(ctx).Where("provider_account_id = ? AND kind = ?", accountID, kind).First(&stream).Error; err != nil {
		return nil, err
	}
	if source != "query" {
		if stream.Suspended {
			return nil, ErrSMSPaused
		}
		var owners int64
		if err := s.db.WithContext(ctx).Model(&model.SMSPollingStream{}).Where("owner_key = ? AND provider_account_id <> ?", appID+":"+kind, accountID).Count(&owners).Error; err != nil {
			return nil, err
		}
		if owners > 0 {
			return nil, fmt.Errorf("%w：此队列已由其他本地账号管理", ErrSMSBusy)
		}
		if source == "poll" && (!stream.Enabled || stream.SDKAppID != appID) {
			return nil, ErrSMSPaused
		}
		// Check the ledger even before its recovery scanner has run.
		var interrupted int64
		if err := s.db.WithContext(ctx).Model(&model.SMSPullRun{}).Where("sdk_app_id = ? AND kind = ? AND source <> ? AND state = ?", appID, kind, "query", "started").Count(&interrupted).Error; err != nil {
			return nil, err
		}
		if interrupted > 0 {
			return nil, ErrSMSPaused
		}
	}
	reporter, err := s.reporter(account)
	if err != nil {
		return nil, err
	}
	run := &model.SMSPullRun{ProviderAccountID: accountID, SDKAppID: appID, Kind: kind, Source: source, State: "started", StartedAt: s.now()}
	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		return nil, err
	}
	var response *senderdomain.SMSEventResponse
	if source == "query" {
		response, err = reporter.QuerySMSEvents(ctx, request)
	} else {
		response, err = reporter.PullSMSEvents(ctx, request)
	}
	if err != nil {
		s.finishFailedRun(run, err)
		return nil, err
	}
	if response == nil {
		err = &senderdomain.RemoteResourceError{Code: "INVALID_RESPONSE", Message: "腾讯云未返回有效响应", Uncertain: source != "query"}
		s.finishFailedRun(run, err)
		return nil, err
	}
	// Use a bounded independent context to save a received response even if the
	// client disconnected. A failed commit leaves the pre-existing ledger open.
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer saveCancel()
	var result *SMSCollectionResult
	err = s.db.WithContext(saveCtx).Transaction(func(tx *gorm.DB) error {
		var insertErr error
		result, insertErr = s.ingestTx(tx, accountID, appID, kind, source, response.RequestID, response.Items)
		if insertErr != nil {
			return insertErr
		}
		now := s.now()
		return tx.Model(run).Updates(map[string]any{"state": "saved", "request_id": response.RequestID, "received": result.Received, "inserted": result.Inserted, "duplicates": result.Duplicates, "finished_at": now}).Error
	})
	if err != nil {
		message := "腾讯云已返回数据，但本地保存失败；已暂停队列，请核对记录并按号码补录"
		if source == "query" {
			message = "腾讯云查询结果未能保存，请重试查询"
		}
		failure := &senderdomain.RemoteResourceError{Code: "PERSISTENCE_FAILED", Message: message, RequestID: response.RequestID, Uncertain: source != "query"}
		s.finishFailedRun(run, failure)
		return nil, failure
	}
	result.RunID, result.RequestID, result.LimitReached = run.ID, response.RequestID, response.LimitReached
	if source != "query" {
		next := s.now().Add(time.Duration(stream.IntervalSeconds) * time.Second)
		if response.LimitReached {
			next = s.now().Add(time.Second)
		}
		if err := s.db.WithContext(saveCtx).Model(&stream).Updates(map[string]any{"next_poll_at": next, "last_success_at": s.now(), "last_run_id": run.ID, "failures": 0, "last_error": "", "updated_at": s.now()}).Error; err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *SMSEventService) finishFailedRun(run *model.SMSPullRun, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	code, uncertain, message := "REQUEST_FAILED", false, err.Error()
	requestID := ""
	var remote *senderdomain.RemoteResourceError
	if errors.As(err, &remote) {
		code, uncertain, requestID = remote.Code, remote.Uncertain, remote.RequestID
	}
	state := "failed"
	if uncertain {
		state = "uncertain"
	}
	if saveErr := s.db.WithContext(ctx).Model(run).Updates(map[string]any{"state": state, "error_code": code, "error_message": message, "request_id": requestID, "finished_at": s.now()}).Error; saveErr != nil {
		s.logger.Error("保存短信拉取失败记录失败：" + saveErr.Error())
	}
	if run.Source == "query" {
		return
	}
	permanent := uncertain || strings.HasPrefix(code, "AuthFailure") || strings.HasPrefix(code, "UnauthorizedOperation") || strings.HasPrefix(code, "UnsupportedOperation") || strings.HasPrefix(code, "InvalidParameter")
	updates := map[string]any{"last_run_id": run.ID, "last_error": message, "failures": gorm.Expr("failures + 1"), "next_poll_at": s.now().Add(time.Minute), "updated_at": s.now()}
	if permanent {
		updates["suspended"], updates["suspend_reason"] = true, message
	}
	if saveErr := s.db.WithContext(ctx).Model(&model.SMSPollingStream{}).Where("provider_account_id = ? AND kind = ?", run.ProviderAccountID, run.Kind).Updates(updates).Error; saveErr != nil {
		s.logger.Error("保存短信轮询状态失败：" + saveErr.Error())
	}
}

func (s *SMSEventService) IngestCallback(ctx context.Context, accountID uint, rows []senderdomain.SMSEvent) (*SMSCollectionResult, error) {
	_, appID, err := s.account(ctx, accountID, false)
	if err != nil {
		return nil, err
	}
	var result *SMSCollectionResult
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		result, e = s.ingestTx(tx, accountID, appID, senderdomain.SMSReports, "callback", "", rows)
		return e
	})
	return result, err
}

func (s *SMSEventService) ingestTx(tx *gorm.DB, accountID uint, appID, kind, source, requestID string, rows []senderdomain.SMSEvent) (*SMSCollectionResult, error) {
	result := &SMSCollectionResult{Received: len(rows), Items: []*model.ProviderSMSEvent{}}
	for _, row := range rows {
		phone := apphelper.ParsePhoneNumber(row.Mobile)
		if phone.Valid {
			row.Mobile = phone.E164
		}
		identity := []any{appID, kind, row.ProviderMsgID, row.Mobile, row.Status}
		if kind == senderdomain.SMSReplies {
			identity = []any{appID, kind, row.Mobile, row.OccurredAt.Unix(), row.Content, row.SignName, row.ExtendCode}
		}
		// Malformed facts have no usable delivery identity; retain them verbatim.
		if !phone.Valid || (kind == senderdomain.SMSReports && row.ProviderMsgID == "") {
			identity = append(identity, row.RawData)
		}
		encoded, _ := json.Marshal(identity)
		hash := sha256.Sum256(encoded)
		now := s.now()
		event := &model.ProviderSMSEvent{EventKey: hex.EncodeToString(hash[:]), ProviderAccountID: accountID, SDKAppID: appID, Kind: kind, Source: source, ProviderMsgID: row.ProviderMsgID, Mobile: row.Mobile, Status: row.Status, ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, Content: row.Content, SignName: row.SignName, ExtendCode: row.ExtendCode, RawData: row.RawData, RequestID: requestID, State: "pending", NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now}
		if !row.OccurredAt.IsZero() {
			at := timeutil.Normalize(row.OccurredAt)
			event.OccurredAt = &at
		}
		if event.RawData == "" {
			body, _ := json.Marshal(row)
			event.RawData = string(body)
		}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_key"}}, DoNothing: true}).Create(event)
		if created.Error != nil {
			return nil, created.Error
		}
		if created.RowsAffected == 0 {
			if err := tx.Where("event_key = ?", event.EventKey).First(event).Error; err != nil {
				return nil, err
			}
			result.Duplicates++
		} else {
			result.Inserted++
		}
		if event.State == "pending" || event.State == "failed" || event.State == "effect_pending" {
			result.Pending++
		}
		result.Items = append(result.Items, event)
	}
	return result, nil
}
