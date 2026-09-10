package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpImpl "cnb.cool/mliev/open/go-web/pkg/server/http_server/impl"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/gin-gonic/gin"
	"github.com/muleiwu/gsr"
)

type resourceErrorLog struct {
	message string
	fields  map[string]any
}

func (*resourceErrorLog) Debug(string, ...gsr.LoggerField)  {}
func (*resourceErrorLog) Info(string, ...gsr.LoggerField)   {}
func (*resourceErrorLog) Notice(string, ...gsr.LoggerField) {}
func (*resourceErrorLog) Warn(string, ...gsr.LoggerField)   {}
func (*resourceErrorLog) Fatal(string, ...gsr.LoggerField)  {}

func (l *resourceErrorLog) Error(message string, fields ...gsr.LoggerField) {
	l.message = message
	l.fields = map[string]any{}
	for _, field := range fields {
		l.fields[field.GetKey()] = field.GetValue()
	}
}

type resourceErrorContext struct {
	httpInterfaces.RouterContextInterface
	request  *http.Request
	reported []error
	status   int
	response dto.Response
}

func (c *resourceErrorContext) Request() *http.Request { return c.request }
func (c *resourceErrorContext) Param(key string) string {
	if key == "id" {
		return "1"
	}
	return ""
}
func (c *resourceErrorContext) GetString(key string) string {
	if key == "traceId" {
		return "01a08b3b-cda3-7c0e-9476-a13305cd69e9"
	}
	return ""
}
func (c *resourceErrorContext) Error(err error) { c.reported = append(c.reported, err) }
func (c *resourceErrorContext) JSON(status int, body any) {
	c.status = status
	c.response = body.(dto.Response)
}

func TestProviderResourceErrorIsRecordedWithRequestTrace(t *testing.T) {
	logger := &resourceErrorLog{}
	registerBindingControllerDependency[gsr.Logger](logger)
	c := &resourceErrorContext{request: httptest.NewRequest(http.MethodPost, "/api/admin/provider-accounts/1/resource-sync/preview", nil)}
	resourceControllerError(c, &domain.RemoteResourceError{Code: "9006", Message: "超频", RequestID: "upstream-123"})
	if len(c.reported) != 1 {
		t.Fatalf("request logger errors are empty: reported=%v", c.reported)
	}
	if !strings.Contains(c.reported[0].Error(), "9006") || !strings.Contains(c.reported[0].Error(), "超频") {
		t.Fatalf("missing provider failure: %v", c.reported)
	}
	if logger.fields["traceId"] != c.GetString("traceId") || logger.fields["provider_error_code"] != "9006" || logger.fields["provider_error_message"] != "超频" {
		t.Fatalf("missing structured diagnostic: %+v", logger)
	}
	if c.status != 502 || c.response.Message != "超频（RequestId: upstream-123）" {
		t.Fatalf("HTTP contract changed: %+v", c.response)
	}
}

func TestProviderResourceValidationDoesNotLogAsUpstreamFailure(t *testing.T) {
	logger := &resourceErrorLog{}
	registerBindingControllerDependency[gsr.Logger](logger)
	c := &resourceErrorContext{request: httptest.NewRequest(http.MethodPost, "/", nil)}
	resourceControllerError(c, errors.New("invalid resource kind"))
	if logger.message != "" || len(c.reported) != 0 || c.status != 400 {
		t.Fatalf("validation mislabeled: %+v %+v", logger, c)
	}
}

func TestProviderResourceGinSummaryAndDiagnosticShareTrace(t *testing.T) {
	logger := &resourceErrorLog{}
	registerBindingControllerDependency[gsr.Logger](logger)
	engine := gin.New()
	var summary string
	engine.Use(func(c *gin.Context) {
		c.Set("traceId", "trace-example")
		c.Next()
		summary = c.Errors.ByType(gin.ErrorTypePrivate).String()
	})
	engine.POST("/api/admin/provider-accounts/:id/resource-sync/preview", httpImpl.NewHttpDeps().WrapHandler(func(c httpInterfaces.RouterContextInterface) {
		resourceControllerError(c, &domain.RemoteResourceError{Code: "9006", Message: "超频", Diagnostic: domain.ResourceDiagnostic{ProviderCode: "zrwinfo_sms", AccountID: 1, Operation: domain.ResourceQuery, API: "/query/templatelist", HTTPStatus: 200, ResponseBody: `{"code":9006,"msg":"超频"}`}}, domain.ResourceTemplates)
	}))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/admin/provider-accounts/1/resource-sync/preview", strings.NewReader(`{"kind":"templates","secret":"request-body-must-not-be-logged"}`)))
	if response.Code != 502 || !strings.Contains(summary, "9006") || !strings.Contains(summary, "超频") {
		t.Fatalf("request summary: %q", summary)
	}
	if logger.fields["traceId"] != "trace-example" || logger.fields["resource_kind"] != "templates" || logger.fields["upstream_status"] != 200 || logger.fields["response_body"] == "" {
		t.Fatalf("diagnostic: %+v", logger.fields)
	}
	if strings.Contains(summary, "request-body-must-not-be-logged") || strings.Contains(response.Body.String(), "response_body") {
		t.Fatal("private diagnostics leaked")
	}
}
