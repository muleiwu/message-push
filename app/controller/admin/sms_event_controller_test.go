package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

type smsControllerContext struct {
	httpInterfaces.RouterContextInterface
	request *http.Request
	params  map[string]string
	status  int
	body    dto.Response
}

func (c *smsControllerContext) Param(name string) string { return c.params[name] }
func (c *smsControllerContext) Request() *http.Request   { return c.request }
func (c *smsControllerContext) ShouldBindJSON(value any) error {
	return json.NewDecoder(c.request.Body).Decode(value)
}
func (c *smsControllerContext) JSON(status int, value any) {
	c.status = status
	c.body = value.(dto.Response)
}

type smsControllerService struct {
	smsEventAdminService
	err          error
	account      uint
	kind, source string
	query        service.SMSQueryRequest
}

func (s *smsControllerService) Collect(_ context.Context, account uint, kind, source string, query service.SMSQueryRequest) (*service.SMSCollectionResult, error) {
	s.account, s.kind, s.source, s.query = account, kind, source, query
	return &service.SMSCollectionResult{RequestID: "request-id", Received: 1, Inserted: 1, Pending: 1}, s.err
}

func TestSMSControllerCollectionContractAndErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
	}{
		{"success", nil, 200}, {"invalid", service.ErrSMSInvalid, 400}, {"busy", service.ErrSMSBusy, 409}, {"paused", service.ErrSMSPaused, 409}, {"not found", service.ErrSMSNotFound, 404}, {"internal", errors.New("storage failed"), 500},
		{"cloud", &domain.RemoteResourceError{Code: "AuthFailure", Message: "无权拉取", RequestID: "cloud-id"}, 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := &smsControllerService{err: test.err}
			c := SMSEventController{newService: func() smsEventAdminService { return s }}
			ctx := &smsControllerContext{request: httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"phone_number":"13800138000","begin_time":10,"end_time":20,"limit":100}`)), params: map[string]string{"id": "17", "event": "reports"}}
			c.Query(ctx)
			if ctx.status != test.status {
				t.Fatalf("status=%d body=%+v", ctx.status, ctx.body)
			}
			if s.account != 17 || s.kind != "reports" || s.source != "query" || s.query.PhoneNumber != "13800138000" {
				t.Fatalf("wrong service arguments: %+v", s)
			}
			if test.name == "cloud" {
				if !strings.Contains(ctx.body.Message, "cloud-id") {
					t.Fatal("request ID missing")
				}
				data := ctx.body.Data.(map[string]any)
				if data["provider_error_code"] != "AuthFailure" {
					t.Fatal("provider code missing")
				}
			}
		})
	}
}

func TestSMSControllerInvalidIdentityNeverResolvesService(t *testing.T) {
	c := SMSEventController{newService: func() smsEventAdminService { t.Fatal("resolved service before validating ID"); return nil }}
	ctx := &smsControllerContext{params: map[string]string{"id": "bad"}}
	c.Pull(ctx)
	if ctx.status != 400 {
		t.Fatalf("invalid ID status=%d", ctx.status)
	}
}
