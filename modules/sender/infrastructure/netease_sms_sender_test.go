package infrastructure

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func TestNeteaseBuildAuthHeaders(t *testing.T) {
	s := NewNeteaseSMSSender()
	appKey := "app_key_xx"
	appSecret := "secret123"

	headers := s.buildAuthHeaders(appKey, appSecret)

	if headers["AppKey"] != appKey {
		t.Errorf("AppKey = %q, want %q", headers["AppKey"], appKey)
	}
	if headers["Content-Type"] != "application/x-www-form-urlencoded;charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", headers["Content-Type"])
	}

	// CheckSum 必须等于 SHA1(AppSecret + Nonce + CurTime)，小写十六进制
	sum := sha1.Sum([]byte(appSecret + headers["Nonce"] + headers["CurTime"]))
	want := hex.EncodeToString(sum[:])
	if headers["CheckSum"] != want {
		t.Errorf("CheckSum = %q, want %q", headers["CheckSum"], want)
	}
	if len(headers["CheckSum"]) != 40 {
		t.Errorf("CheckSum length = %d, want 40", len(headers["CheckSum"]))
	}
}

func TestBuildParamsFromMapping(t *testing.T) {
	s := NewNeteaseSMSSender()

	tests := []struct {
		name     string
		template string
		params   map[string]string
		want     []string
	}{
		{
			name:     "ordered by placeholder appearance",
			template: "您的验证码是{code}，{minute}分钟内有效",
			params:   map[string]string{"minute": "5", "code": "123456"},
			want:     []string{"123456", "5"},
		},
		{
			name:     "missing param becomes empty",
			template: "{a}{b}{c}",
			params:   map[string]string{"a": "1", "c": "3"},
			want:     []string{"1", "", "3"},
		},
		{
			name:     "no params returns nil",
			template: "{code}",
			params:   map[string]string{},
			want:     nil,
		},
		{
			name:     "no placeholders returns nil",
			template: "纯文本无占位符",
			params:   map[string]string{"x": "1"},
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.buildParamsFromMapping(tt.template, tt.params)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildParamsFromMapping() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeteaseSendCode(t *testing.T) {
	t.Run("success builds sendcode request and parses sendid", func(t *testing.T) {
		var gotForm url.Values
		var gotHeaders http.Header
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotForm = r.PostForm
			gotHeaders = r.Header
			w.Header().Set("Content-Type", "application/json")
			// 真实响应：msg 为 sendid，obj 为验证码（与 sendtemplate 相反）
			_, _ = w.Write([]byte(`{"code":200,"msg":"2202","obj":"123214"}`))
		}))
		defer srv.Close()

		old := neteaseSendCodeURL
		neteaseSendCodeURL = srv.URL
		defer func() { neteaseSendCodeURL = old }()

		s := NewNeteaseSMSSender()
		task := &model.PushTask{TaskID: "t1", Receiver: "18355190731"}
		resp := s.sendCodeOne(context.Background(), "ak", "sk", "27194667", task, map[string]string{"code": "123214"})

		if !resp.Success {
			t.Fatalf("expected success, got %+v", resp)
		}
		if resp.ProviderID != "2202" {
			t.Errorf("ProviderID = %q, want 2202 (msg 字段的 sendid，而非 obj 的验证码)", resp.ProviderID)
		}
		if resp.Status != constants.TaskStatusSent {
			t.Errorf("Status = %q, want %q", resp.Status, constants.TaskStatusSent)
		}

		// 验证码接口为单手机号 + paramMap 对象
		if gotForm.Get("mobile") != "18355190731" {
			t.Errorf("mobile = %q, want 18355190731", gotForm.Get("mobile"))
		}
		if gotForm.Get("templateid") != "27194667" {
			t.Errorf("templateid = %q, want 27194667", gotForm.Get("templateid"))
		}
		if gotForm.Get("paramMap") != `{"code":"123214"}` {
			t.Errorf("paramMap = %q, want {\"code\":\"123214\"}", gotForm.Get("paramMap"))
		}
		// 不应携带模板短信的数组参数
		if gotForm.Has("mobiles") || gotForm.Has("params") {
			t.Errorf("sendcode request must not contain mobiles/params arrays: %v", gotForm)
		}
		// 鉴权头齐全
		if gotHeaders.Get("AppKey") != "ak" || gotHeaders.Get("CheckSum") == "" ||
			gotHeaders.Get("Nonce") == "" || gotHeaders.Get("CurTime") == "" {
			t.Errorf("missing auth headers: %v", gotHeaders)
		}
	})

	t.Run("business error 404 template id not exist", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":404,"msg":"template id not exist"}`))
		}))
		defer srv.Close()

		old := neteaseSendCodeURL
		neteaseSendCodeURL = srv.URL
		defer func() { neteaseSendCodeURL = old }()

		s := NewNeteaseSMSSender()
		task := &model.PushTask{TaskID: "t2", Receiver: "18355190731"}
		resp := s.sendCodeOne(context.Background(), "ak", "sk", "999", task, map[string]string{"code": "1"})

		if resp.Success {
			t.Fatalf("expected failure, got success: %+v", resp)
		}
		if resp.ErrorCode != "404" {
			t.Errorf("ErrorCode = %q, want 404", resp.ErrorCode)
		}
		if resp.ErrorMessage != "template id not exist" {
			t.Errorf("ErrorMessage = %q, want 'template id not exist'", resp.ErrorMessage)
		}
		if resp.ProviderID != "" {
			t.Errorf("ProviderID = %q, want empty on failure", resp.ProviderID)
		}
	})
}

func TestNeteaseBatchSendSharesRealSendID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"ok","obj":1490}`))
	}))
	defer srv.Close()

	old := neteaseSendTemplateURL
	neteaseSendTemplateURL = srv.URL
	defer func() { neteaseSendTemplateURL = old }()

	s := NewNeteaseSMSSender()
	req := &domain.BatchSendRequest{
		ProviderAccount: &model.ProviderAccount{Config: `{"app_key":"ak","app_secret":"sk"}`},
		Tasks: []*model.PushTask{
			{TaskID: "t1", Receiver: "13800000001", TemplateCode: "27194667"},
			{TaskID: "t2", Receiver: "13800000002", TemplateCode: "27194667"},
		},
	}

	resp, err := s.BatchSend(context.Background(), req)
	if err != nil {
		t.Fatalf("BatchSend error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(resp.Results))
	}
	// 整批共用同一真实 sendid，不再带 _0/_1 后缀（否则回执无法关联）
	for i, r := range resp.Results {
		if r.ProviderID != "1490" {
			t.Errorf("results[%d].ProviderID = %q, want 1490 (real shared sendid, no suffix)", i, r.ProviderID)
		}
	}
}

func TestNeteaseHandleCallback(t *testing.T) {
	s := NewNeteaseSMSSender()

	t.Run("downlink receipt delivered and failed", func(t *testing.T) {
		body := `{"eventType":"11","objects":[
			{"mobile":"13800000000","sendid":"1490","result":"DELIVRD","reportTime":"2017-06-06 10:40:30","spliced":"1","templateId":1234},
			{"mobile":"13800000001","sendid":"1491","result":"UNDELIV","reportTime":"2017-06-06 10:41:30","reason":"空号"}
		]}`
		resp, results, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte(body)})
		if err != nil {
			t.Fatalf("HandleCallback error: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("status code = %d, want 200", resp.StatusCode)
		}
		if len(results) != 2 {
			t.Fatalf("results len = %d, want 2", len(results))
		}
		if results[0].ProviderID != "1490" || results[0].Status != constants.CallbackStatusDelivered {
			t.Errorf("result[0] = %+v, want delivered 1490", results[0])
		}
		if results[0].Type != constants.CallbackTypeReport || results[0].Mobile != "13800000000" {
			t.Errorf("result[0] = %+v, want report type with mobile", results[0])
		}
		if results[1].Status != constants.CallbackStatusFailed || results[1].ErrorMessage != "空号" {
			t.Errorf("result[1] = %+v, want failed with reason", results[1])
		}
	})

	t.Run("downlink receipt with numeric eventType and spliced", func(t *testing.T) {
		// 真实回调报文：文档中 eventType/spliced 为字符串，实际下发为数字
		body := `{"objects":[{"result":"DELIVRD","reason":"已送达","sendid":"2202","spliced":1,"mobile":"15385390860","templateId":27194667,"sendTime":"2026-07-14 18:41:20","reportTime":"2026-07-14 18:41:32"}],"eventType":11}`
		_, results, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte(body)})
		if err != nil {
			t.Fatalf("HandleCallback error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("results len = %d, want 1", len(results))
		}
		if results[0].ProviderID != "2202" || results[0].Status != constants.CallbackStatusDelivered {
			t.Errorf("result = %+v, want delivered 2202", results[0])
		}
		if results[0].Mobile != "15385390860" {
			t.Errorf("mobile = %q, want 15385390860", results[0].Mobile)
		}
	})

	t.Run("uplink message parsed as upstream", func(t *testing.T) {
		body := `{"eventType":"12","objects":[{"mobile":"13800000000","content":"TD","receiveTime":"2017-06-06 10:40:30"}]}`
		_, results, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte(body)})
		if err != nil {
			t.Fatalf("HandleCallback error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("uplink results len = %d, want 1", len(results))
		}
		if results[0].Type != constants.CallbackTypeUpstream {
			t.Errorf("type = %q, want upstream", results[0].Type)
		}
		if results[0].Mobile != "13800000000" || results[0].Content != "TD" {
			t.Errorf("result = %+v, want mobile 13800000000 content TD", results[0])
		}
		if results[0].ReceiveTime.IsZero() {
			t.Error("receiveTime should be parsed")
		}
	})

	t.Run("invalid body returns error", func(t *testing.T) {
		_, _, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte("not json")})
		if err == nil {
			t.Error("expected error for invalid body, got nil")
		}
	})
}
