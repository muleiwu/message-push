package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func oneBotTestRequest(t *testing.T, config map[string]any) *domain.SendRequest {
	t.Helper()
	account := &model.ProviderAccount{ProviderCode: constants.ProviderOneBot, ProviderType: constants.MessageTypeQQ}
	if err := account.SetConfig(config); err != nil {
		t.Fatal(err)
	}
	return &domain.SendRequest{
		ProviderAccount: account,
		Task:            &model.PushTask{TaskID: "onebot-task", MessageType: constants.MessageTypeQQ, Receiver: "private:123456"},
		RenderedContent: "你好，世界！\n[CQ:at,qq=123456]",
	}
}

func TestOneBotHTTPContract(t *testing.T) {
	for _, tt := range []struct {
		name, path, receiver, format, token string
		escape                              bool
	}{
		{"private default", "", "private:123456", "", "", true},
		{"group text", "/", "group:987654", "text", "test-token", true},
		{"group CQ", "/proxy/onebot/", "group:987654", "cqcode", "test-token", false},
		{"private CQ prefix", "/proxy/onebot", "private:9223372036854775807", "cqcode", "test-token", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" || r.URL.Path != strings.TrimRight(tt.path, "/")+"/send_msg" || r.URL.RawQuery != "" {
					t.Errorf("wrong request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing JSON content type")
				}
				wantAuth := ""
				if tt.token != "" {
					wantAuth = "Bearer " + tt.token
				}
				if r.Header.Get("Authorization") != wantAuth {
					t.Error("wrong authorization")
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				var content string
				if err := json.Unmarshal(body["message"], &content); err != nil || content != "你好，世界！\n[CQ:at,qq=123456]" {
					t.Errorf("content was altered: %q %v", content, err)
				}
				kind, number, _ := strings.Cut(tt.receiver, ":")
				idField, excluded := "user_id", "group_id"
				if kind == "group" {
					idField, excluded = excluded, idField
				}
				if string(body[idField]) != number || body[excluded] != nil || string(body["message_type"]) != fmt.Sprintf("%q", kind) {
					t.Errorf("wrong destination: %s", body)
				}
				if string(body["auto_escape"]) != fmt.Sprint(tt.escape) {
					t.Error("incorrect auto_escape")
				}
				fmt.Fprint(w, `{"status":"ok","retcode":0,"data":{"message_id":-12345}}`)
			}))
			defer server.Close()
			config := map[string]any{"base_url": server.URL + tt.path, "access_token": tt.token}
			if tt.format != "" {
				config["message_format"] = tt.format
			}
			req := oneBotTestRequest(t, config)
			req.Task.Receiver = tt.receiver
			s := NewOneBotSender()
			if s.client.Timeout != 10*time.Second {
				t.Fatal("unexpected default timeout")
			}
			got, err := s.Send(context.Background(), req)
			if err != nil || !got.Success || got.Status != constants.TaskStatusSuccess || got.ProviderID != "-12345" || got.TaskID != req.Task.TaskID {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			if !json.Valid([]byte(got.RequestData)) || !json.Valid([]byte(got.ResponseData)) || calls.Load() != 1 {
				t.Fatalf("missing diagnostics or repeated send: %+v", got)
			}
			if tt.token != "" && strings.Contains(got.RequestData+got.ResponseData, tt.token) {
				t.Fatal("token in diagnostics")
			}
		})
	}
}

func TestOneBotResponseValidation(t *testing.T) {
	for _, tt := range []struct {
		name       string
		status     int
		body, code string
	}{
		{"business failure", 200, `{"status":"failed","retcode":1404,"data":null,"wording":"群不存在"}`, "1404"},
		{"async", 200, `{"status":"async","retcode":1,"data":null}`, constants.ErrorCodeOneBotAsyncUnsupported},
		{"async missing code", 200, `{"status":"async","data":null}`, constants.ErrorCodeOneBotAsyncUnsupported},
		{"async with nonstandard code", 200, `{"status":"async","retcode":"1","data":null}`, constants.ErrorCodeOneBotAsyncUnsupported},
		{"async code", 200, `{"status":"ok","retcode":1,"data":null}`, constants.ErrorCodeOneBotAsyncUnsupported},
		{"missing status", 200, `{"retcode":0,"data":{"message_id":1}}`, "ONEBOT_INVALID_RESPONSE"},
		{"missing code", 200, `{"status":"ok","data":{"message_id":1}}`, "ONEBOT_INVALID_RESPONSE"},
		{"null code", 200, `{"status":"ok","retcode":null,"data":{"message_id":1}}`, "ONEBOT_INVALID_RESPONSE"},
		{"missing data", 200, `{"status":"ok","retcode":0}`, "ONEBOT_INVALID_RESPONSE"},
		{"missing id", 200, `{"status":"ok","retcode":0,"data":{}}`, "ONEBOT_INVALID_RESPONSE"},
		{"null id", 200, `{"status":"ok","retcode":0,"data":{"message_id":null}}`, "ONEBOT_INVALID_RESPONSE"},
		{"invalid id", 200, `{"status":"ok","retcode":0,"data":{"message_id":"bad"}}`, "ONEBOT_INVALID_RESPONSE"},
		{"fractional id", 200, `{"status":"ok","retcode":0,"data":{"message_id":1.5}}`, "ONEBOT_INVALID_RESPONSE"},
		{"overflow id", 200, `{"status":"ok","retcode":0,"data":{"message_id":2147483648}}`, "ONEBOT_INVALID_RESPONSE"},
		{"inconsistent success", 200, `{"status":"ok","retcode":42,"data":{"message_id":1}}`, "ONEBOT_INVALID_RESPONSE"},
		{"inconsistent failure", 200, `{"status":"failed","retcode":0}`, "ONEBOT_INVALID_RESPONSE"},
		{"malformed", 200, `not JSON`, "ONEBOT_INVALID_RESPONSE"},
		{"empty", 200, ``, "ONEBOT_INVALID_RESPONSE"},
		{"unauthorized", 401, `Unauthorized`, "HTTP_401"},
		{"forbidden", 403, `{"message":"Forbidden"}`, "HTTP_403"},
		{"wrong action", 404, `Not found`, "HTTP_404"},
		{"HTTP failure with success body", 500, `{"status":"ok","retcode":0,"data":{"message_id":1}}`, "HTTP_500"},
		{"too large", 200, strings.Repeat("x", (1<<20)+1), "ONEBOT_INVALID_RESPONSE"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tt.status); fmt.Fprint(w, tt.body) }))
			defer server.Close()
			got, err := NewOneBotSender().Send(context.Background(), oneBotTestRequest(t, map[string]any{"base_url": server.URL}))
			if err != nil || got.Success || got.Status != constants.TaskStatusFailed || got.ErrorCode != tt.code || got.ErrorMessage == "" {
				t.Fatalf("result=%+v err=%v", got, err)
			}
			if tt.name == "business failure" && got.ErrorMessage != "群不存在" {
				t.Fatal(got.ErrorMessage)
			}
			if got.ResponseData != "" && !json.Valid([]byte(got.ResponseData)) {
				t.Fatal("response diagnostics cannot be stored in JSON columns")
			}
		})
	}
}

func TestOneBotRejectsInvalidRequestsBeforeSending(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1) }))
	defer server.Close()
	for _, config := range []map[string]any{
		{}, {"base_url": "file:///tmp/onebot"}, {"base_url": "http://"}, {"base_url": "://bad"},
		{"base_url": server.URL + "?access_token=secret"}, {"base_url": "http://user:secret@localhost"},
		{"base_url": server.URL + "#fragment"}, {"base_url": server.URL, "message_format": "json"},
		{"base_url": server.URL, "message_format": true}, {"base_url": server.URL, "access_token": true},
		{"base_url": server.URL, "access_token": "secret\n"},
	} {
		got, err := NewOneBotSender().Send(context.Background(), oneBotTestRequest(t, config))
		if err != nil || got.Success || got.ErrorCode != "ONEBOT_INVALID_CONFIG" {
			t.Fatalf("invalid config result=%+v err=%v", got, err)
		}
	}
	for _, receiver := range []string{"", "123456", "group:0", "private:-1", "group:123,456", "group:9223372036854775808"} {
		req := oneBotTestRequest(t, map[string]any{"base_url": server.URL})
		req.Task.Receiver = receiver
		got, _ := NewOneBotSender().Send(context.Background(), req)
		if got.ErrorCode != "ONEBOT_INVALID_RECEIVER" {
			t.Fatalf("invalid receiver accepted: %+v", got)
		}
	}
	req := oneBotTestRequest(t, map[string]any{"base_url": server.URL})
	req.RenderedContent = " \n"
	got, _ := NewOneBotSender().Send(context.Background(), req)
	if got.ErrorCode != "ONEBOT_EMPTY_MESSAGE" || calls.Load() != 0 {
		t.Fatal("invalid request contacted upstream")
	}
}

func TestOneBotRedactsEchoedToken(t *testing.T) {
	token := "sensitive-\"token<&>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "failed", "retcode": 403, "wording": "Bearer " + token, "debug": url.QueryEscape(token)})
	}))
	defer server.Close()
	got, _ := NewOneBotSender().Send(context.Background(), oneBotTestRequest(t, map[string]any{"base_url": server.URL, "access_token": token}))
	encoded, _ := json.Marshal(token)
	for _, secret := range []string{token, string(encoded[1 : len(encoded)-1]), url.QueryEscape(token)} {
		if strings.Contains(got.ErrorMessage+got.RequestData+got.ResponseData, secret) {
			t.Fatal("token leaked in diagnostics")
		}
	}
	if !strings.Contains(got.ResponseData, "[redacted]") || !json.Valid([]byte(got.ResponseData)) {
		t.Fatalf("bad redaction: %+v", got)
	}
}

func TestOneBotDiagnosticsRedactDecodedStringsAndPreserveNumbers(t *testing.T) {
	got := oneBotDiagnostic(`{"wording":"Bearer s\u0065cret","data":{"message_id":9223372036854775807}}`, "secret")
	if strings.Contains(got, "secret") || !strings.Contains(got, "[redacted]") || !strings.Contains(got, "9223372036854775807") {
		t.Fatalf("invalid diagnostics: %s", got)
	}
}

func TestOneBotTimeoutCancellationAndRedirect(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%v", cancelRequest), func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := NewOneBotSender()
			s.client.Timeout = 50 * time.Millisecond
			if cancelRequest {
				go func() { <-started; cancel() }()
			}
			got, err := s.Send(ctx, oneBotTestRequest(t, map[string]any{"base_url": server.URL}))
			if err != nil || got.Success || got.ErrorCode != "ONEBOT_HTTP_ERROR" {
				t.Fatalf("result=%+v err=%v", got, err)
			}
		})
	}
	var redirected atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/other" {
			redirected.Add(1)
		}
		http.Redirect(w, r, "/other", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	got, _ := NewOneBotSender().Send(context.Background(), oneBotTestRequest(t, map[string]any{"base_url": server.URL}))
	if got.ErrorCode != "HTTP_307" || redirected.Load() != 0 {
		t.Fatal("redirect replayed the send")
	}
}

func TestOneBotHTTPSAndRegistration(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":"ok","retcode":0,"data":{"message_id":0}}`)
	}))
	defer server.Close()
	s := NewOneBotSender()
	s.client.Transport = server.Client().Transport
	got, err := s.Send(context.Background(), oneBotTestRequest(t, map[string]any{"base_url": server.URL}))
	if err != nil || !got.Success || got.ProviderID != "0" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	meta, err := domain.GetByCode(constants.ProviderOneBot)
	if err != nil || meta.Type != constants.MessageTypeQQ || !meta.SupportsSend || meta.SupportsBatchSend || meta.RequiresSignature || meta.SupportsCallback || meta.SupportsStatusQuery || meta.SupportsStatusPull {
		t.Fatalf("unexpected capabilities: %+v %v", meta, err)
	}
	if _, err := NewFactory().GetSender(constants.ProviderOneBot); err != nil {
		t.Fatal(err)
	}
}
