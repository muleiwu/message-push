package autoload_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/container"
	configImpl "cnb.cool/mliev/open/go-web/pkg/server/config/impl"
	httpImpl "cnb.cool/mliev/open/go-web/pkg/server/http_server/impl"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/config/autoload"
	"cnb.cool/mliev/push/message-push/migrations"
	"cnb.cool/mliev/push/message-push/modules/channel"
	channelAssembly "cnb.cool/mliev/push/message-push/modules/channel/assembly"
	"cnb.cool/mliev/push/message-push/modules/delivery"
	"cnb.cool/mliev/push/message-push/modules/messaging"
	messagingImpl "cnb.cool/mliev/push/message-push/modules/messaging/infrastructure"
	"cnb.cool/mliev/push/message-push/modules/quota"
	_ "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	"cnb.cool/mliev/push/message-push/modules/template"
	templateImpl "cnb.cool/mliev/push/message-push/modules/template/infrastructure"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPublicChannelCatalogHTTP(t *testing.T) {
	db, app, channelID, secret := newCatalogHTTPFixture(t)
	registerCatalogDependency[*gorm.DB](db)
	registerCatalogDependency[gsr.Logger](catalogHTTPLogger{})
	registerCatalogDependency[gsr.Provider](configImpl.NewConfig())
	quotaService := &catalogQuota{used: 1}
	registerCatalogDependency[quota.Service](quotaService)
	rate := &catalogRedisHook{}
	redisClient := redis.NewClient(&redis.Options{Addr: "unused.invalid:6379"})
	redisClient.AddHook(rate)
	t.Cleanup(func() { _ = redisClient.Close() })
	registerCatalogDependency[*redis.Client](redisClient)
	catalogAssembly := &channelAssembly.Catalog{}
	catalogService, err := catalogAssembly.Assembly()
	if err != nil {
		t.Fatal(err)
	}
	container.Register(container.NewSimpleProvider(catalogAssembly.Type(), catalogService))
	registerCatalogDependency[channel.Selector](&catalogUnusedSelector{})
	producer := &catalogProducer{}
	registerCatalogDependency[delivery.Producer](producer)
	registerCatalogDependency[template.Renderer](templateImpl.NewTemplateHelper())
	registerCatalogDependency[messaging.Service](messagingImpl.NewMessageService())

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerRoutes := autoload.Router{}.InitConfig()["http.router"].(func(httpInterfaces.RouterInterface))
	registerRoutes(httpImpl.NewRouter(engine, httpImpl.NewHttpDeps()))

	request := func(t *testing.T, method, target string, body any) *http.Request {
		t.Helper()
		var raw []byte
		var params map[string]interface{}
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &params); err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, target, bytes.NewReader(raw))
		timestamp := time.Now().Unix()
		nonce := "catalog-test-nonce"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-App-Id", app.AppID)
		req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
		req.Header.Set("X-Nonce", nonce)
		req.Header.Set("X-Signature", helper.NewSignatureHelper().GenerateSignature(secret, method, req.URL.Path, params, timestamp, nonce))
		return req
	}
	serve := func(t *testing.T, req *http.Request, wantStatus, wantCode int) json.RawMessage {
		t.Helper()
		writer := httptest.NewRecorder()
		engine.ServeHTTP(writer, req)
		var response struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
			t.Fatalf("invalid response: %s: %v", writer.Body.String(), err)
		}
		if writer.Code != wantStatus || response.Code != wantCode {
			t.Fatalf("%s %s: HTTP=%d body=%s, want HTTP=%d code=%d", req.Method, req.URL, writer.Code, writer.Body.String(), wantStatus, wantCode)
		}
		return response.Data
	}
	var selected dto.PublicChannelDetailResponse

	t.Run("catalog stays readable after sending quota is exhausted", func(t *testing.T) {
		var page dto.PublicChannelListResponse
		if err := json.Unmarshal(serve(t, request(t, "GET", "/api/v1/channels?type=sms", nil), 200, 0), &page); err != nil {
			t.Fatal(err)
		}
		if page.Page != 1 || page.Size != 20 || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != channelID {
			t.Fatalf("catalog page = %+v", page)
		}
		if err := json.Unmarshal(serve(t, request(t, "GET", fmt.Sprintf("/api/v1/channels/%d", page.Items[0].ID), nil), 200, 0), &selected); err != nil {
			t.Fatal(err)
		}
		if selected.Template == nil || selected.Readiness.State != "ready" || !selected.SignatureRequired || len(selected.SignatureNames) != 1 {
			t.Fatalf("channel detail = %+v", selected)
		}
		serve(t, request(t, "POST", "/api/v1/messages", map[string]any{"channel_id": channelID, "receiver": "13800138000"}), 200, constants.CodeQuotaExceeded)
		serve(t, request(t, "GET", "/api/v1/channels", nil), 200, 0)
		if quotaService.calls != 1 || quotaService.used != 1 || len(producer.tasks) != 0 {
			t.Fatalf("queries touched sending state: quota=%+v tasks=%d", quotaService, len(producer.tasks))
		}
	})

	t.Run("real send accepts catalog selections", func(t *testing.T) {
		if selected.Template == nil || len(selected.SignatureNames) == 0 {
			t.Fatal("catalog selection is unavailable")
		}
		quotaService.used = 0
		params := make(map[string]string)
		for _, variable := range selected.Template.Variables {
			params[variable] = "123456"
		}
		body := dto.SendRequest{ChannelID: selected.ID, Receiver: "13800138000", SignatureName: selected.SignatureNames[0], TemplateParams: params}
		serve(t, request(t, "POST", "/api/v1/messages", body), 200, 0)
		if len(producer.tasks) != 1 || producer.tasks[0].ChannelID != selected.ID || producer.tasks[0].Signature != body.SignatureName || producer.tasks[0].AppID != app.AppID {
			t.Fatalf("queued tasks = %+v", producer.tasks)
		}
		var saved model.PushTask
		if err := db.Where("task_id = ?", producer.tasks[0].TaskID).First(&saved).Error; err != nil {
			t.Fatal(err)
		}
		var savedParams map[string]string
		if err := json.Unmarshal([]byte(saved.TemplateParams), &savedParams); err != nil || !reflect.DeepEqual(savedParams, params) {
			t.Fatalf("saved params=%v err=%v", savedParams, err)
		}
		if quotaService.used != 1 || quotaService.calls != 2 {
			t.Fatalf("send quota = %+v", quotaService)
		}
	})

	t.Run("invalid query and channel IDs", func(t *testing.T) {
		for _, target := range []string{"/api/v1/channels?page=0", "/api/v1/channels?page=-1", "/api/v1/channels?page_size=0", "/api/v1/channels?page_size=101", "/api/v1/channels?page=abc", "/api/v1/channels?type=unknown", "/api/v1/channels?page=9223372036854775807&page_size=100", "/api/v1/channels/0", "/api/v1/channels/-1", "/api/v1/channels/nope", "/api/v1/channels/18446744073709551616"} {
			serve(t, request(t, "GET", target, nil), 400, 400)
		}
		serve(t, request(t, "GET", "/api/v1/channels/999999", nil), 404, 404)
	})

	t.Run("authentication and rate limiting remain enforced", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*http.Request)
			code   int
		}{
			{name: "missing header", mutate: func(r *http.Request) { r.Header.Del("X-Nonce") }, code: constants.CodeUnauthorized},
			{name: "bad signature", mutate: func(r *http.Request) { r.Header.Set("X-Signature", "bad") }, code: constants.CodeInvalidSignature},
			{name: "expired timestamp", mutate: func(r *http.Request) { r.Header.Set("X-Timestamp", "1") }, code: constants.CodeInvalidSignature},
			{name: "malformed timestamp", mutate: func(r *http.Request) { r.Header.Set("X-Timestamp", "bad") }, code: constants.CodeInvalidTimestamp},
			{name: "unknown application", mutate: func(r *http.Request) { r.Header.Set("X-App-Id", "missing") }, code: constants.CodeInvalidAppID},
			{name: "signature for wrong path", mutate: func(r *http.Request) { r.URL.Path = fmt.Sprintf("/api/v1/channels/%d", channelID) }, code: constants.CodeInvalidSignature},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				req := request(t, "GET", "/api/v1/channels", nil)
				test.mutate(req)
				serve(t, req, 200, test.code)
			})
		}
		if err := db.Model(app).Update("status", 0).Error; err != nil {
			t.Fatal(err)
		}
		serve(t, request(t, "GET", "/api/v1/channels", nil), 200, constants.CodeAppDisabled)
		if err := db.Model(app).Update("status", 1).Error; err != nil {
			t.Fatal(err)
		}
		rate.blocked = true
		serve(t, request(t, "GET", "/api/v1/channels", nil), 200, constants.CodeRateLimitExceeded)
		rate.blocked = false
		if quotaService.calls != 2 {
			t.Fatalf("catalog invoked quota %d times", quotaService.calls)
		}
	})

	t.Run("configuration changes are visible without cache", func(t *testing.T) {
		if err := db.Model(&model.Channel{}).Where("id = ?", channelID).Update("status", 0).Error; err != nil {
			t.Fatal(err)
		}
		serve(t, request(t, "GET", fmt.Sprintf("/api/v1/channels/%d", channelID), nil), 404, 404)
		var page dto.PublicChannelListResponse
		if err := json.Unmarshal(serve(t, request(t, "GET", "/api/v1/channels", nil), 200, 0), &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 0 || page.Items == nil || len(page.Items) != 0 {
			t.Fatalf("disabled channel still visible: %+v", page)
		}
	})

	t.Run("database errors use HTTP 500", func(t *testing.T) {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		// Auth still needs its application query, so fail only catalog queries.
		if err := db.Callback().Query().Before("gorm:query").Register("catalog_database_error", func(tx *gorm.DB) {
			if tx.Statement.Table == "channels" {
				tx.AddError(errors.New("private database error"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		serve(t, request(t, "GET", "/api/v1/channels", nil), 500, 500)
		serve(t, request(t, "GET", fmt.Sprintf("/api/v1/channels/%d", channelID), nil), 500, 500)
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func registerCatalogDependency[T any](value T) {
	container.Register(container.NewSimpleProvider(reflect.TypeFor[T](), value))
}

func newCatalogHTTPFixture(t *testing.T) (*gorm.DB, *model.Application, uint, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "catalog.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	files, err := fs.Sub(migrations.FS(), "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENCRYPTION_KEY", "01234567890123456789012345678901")
	if err := helper.InitCryptoHelper(); err != nil {
		t.Fatal(err)
	}
	const secret = "test-app-secret"
	encrypted, err := helper.EncryptAppSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	app := &model.Application{AppID: "catalog-test", AppName: "catalog test", AppSecret: encrypted, Status: 1, DailyQuota: 1, RateLimit: 100}
	systemTemplate := &model.MessageTemplate{TemplateName: "验证码", ContentType: "text", Content: "您的验证码是{code}，{expire}分钟有效。", Variables: `["code","expire"]`, Status: 1}
	account := &model.ProviderAccount{AccountCode: "catalog-test", AccountName: "internal account", ProviderCode: constants.ProviderAliyunSMS, ProviderType: "sms", Status: 1, Config: `{}`}
	for _, record := range []any{app, systemTemplate, account} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	channel := &model.Channel{Name: "验证码通道", Type: "sms", MessageTemplateID: systemTemplate.ID, Status: 1}
	providerTemplate := &model.ProviderTemplate{ProviderID: account.ID, TemplateName: "provider template", TemplateCode: "SMS_TEST", Variables: `["code"]`, Status: 1}
	signature := &model.ProviderSignature{ProviderAccountID: account.ID, SignatureCode: "实际签名", SignatureName: "internal name", Status: 1}
	for _, record := range []any{channel, providerTemplate, signature} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	binding := &model.ChannelTemplateBinding{ChannelID: channel.ID, ProviderID: account.ID, ProviderTemplateID: providerTemplate.ID, Weight: 10, Priority: 100, Status: 1, IsActive: 1}
	mapping := &model.ChannelSignatureMapping{ChannelID: channel.ID, ProviderID: account.ID, ProviderSignatureID: signature.ID, SignatureName: "验证码", Status: 1}
	for _, record := range []any{binding, mapping} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, app, channel.ID, secret
}

type catalogUnusedSelector struct{ channel.Selector }

type catalogProducer struct {
	delivery.Producer
	tasks []*model.PushTask
}

func (p *catalogProducer) Push(_ context.Context, task *model.PushTask) error {
	p.tasks = append(p.tasks, task)
	return nil
}

type catalogQuota struct {
	quota.Service
	used  int
	calls int
}

func (q *catalogQuota) Check(_ context.Context, _ uint, limit int) (bool, error) {
	q.calls++
	if q.used >= limit {
		return false, nil
	}
	q.used++
	return true, nil
}

// External Redis is replaced at its command boundary; the production middleware runs unchanged.
type catalogRedisHook struct{ blocked bool }

func (*catalogRedisHook) DialHook(redis.DialHook) redis.DialHook {
	return func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("unexpected network access")
	}
}

func (h *catalogRedisHook) ProcessHook(redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, command redis.Cmder) error {
		if command.Name() != "evalsha" {
			return fmt.Errorf("unexpected Redis command: %s", command.Name())
		}
		value := int64(1)
		if h.blocked {
			value = 0
		}
		command.(*redis.Cmd).SetVal(value)
		return nil
	}
}

func (*catalogRedisHook) ProcessPipelineHook(redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(context.Context, []redis.Cmder) error { return errors.New("unexpected Redis pipeline") }
}

type catalogHTTPLogger struct{}

func (catalogHTTPLogger) Debug(string, ...gsr.LoggerField)  {}
func (catalogHTTPLogger) Info(string, ...gsr.LoggerField)   {}
func (catalogHTTPLogger) Notice(string, ...gsr.LoggerField) {}
func (catalogHTTPLogger) Error(string, ...gsr.LoggerField)  {}
func (catalogHTTPLogger) Warn(string, ...gsr.LoggerField)   {}
func (catalogHTTPLogger) Fatal(string, ...gsr.LoggerField)  {}
