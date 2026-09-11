package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestOneBotOptionalSignature(t *testing.T) {
	const body = "服务已恢复[CQ:at,qq=42]"
	for _, receiver := range []string{"private:123", "group:456"} {
		for _, tt := range []struct {
			name, alias, format, want, code string
			signature                       *model.ProviderSignature
		}{
			{name: "unsigned", want: body},
			{name: "empty alias ignores configured signature", alias: "  ", signature: &model.ProviderSignature{SignatureCode: "木雷科技"}, want: body},
			{name: "mapped value", alias: "notice", signature: &model.ProviderSignature{SignatureCode: "木雷科技"}, want: "【木雷科技】" + body},
			{name: "trim signature", alias: "notice", signature: &model.ProviderSignature{SignatureCode: " 木雷科技\n"}, want: "【木雷科技】" + body},
			{name: "already bracketed", alias: "notice", signature: &model.ProviderSignature{SignatureCode: " 【木雷科技】 "}, want: "【木雷科技】" + body},
			{name: "CQ body", alias: "notice", format: "cqcode", signature: &model.ProviderSignature{SignatureCode: "木雷科技"}, want: "【木雷科技】" + body},
			{name: "CQ signature stays literal", alias: "notice", format: "cqcode", signature: &model.ProviderSignature{SignatureCode: "A&[CQ:at,qq=all]"}, want: "【A&amp;&#91;CQ:at,qq=all&#93;】" + body},
			{name: "text signature stays literal", alias: "notice", signature: &model.ProviderSignature{SignatureCode: "A&[CQ:at,qq=all]"}, want: "【A&[CQ:at,qq=all]】" + body},
			{name: "missing mapping", alias: "notice", code: "ONEBOT_INVALID_SIGNATURE"},
			{name: "empty mapping value", alias: "notice", signature: &model.ProviderSignature{SignatureCode: " \n"}, code: "ONEBOT_INVALID_SIGNATURE"},
		} {
			t.Run(receiver+"/"+tt.name, func(t *testing.T) {
				messages := make(chan string, 2)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload struct {
						Message string `json:"message"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					messages <- payload.Message
					fmt.Fprint(w, `{"status":"ok","retcode":0,"data":{"message_id":42}}`)
				}))
				defer server.Close()
				req := oneBotTestRequest(t, map[string]any{"base_url": server.URL, "message_format": tt.format})
				req.Task.Receiver, req.Task.Signature = receiver, tt.alias
				req.Signature, req.RenderedContent = tt.signature, body
				s := NewOneBotSender()
				for i := 0; i < 2; i++ {
					got, err := s.Send(context.Background(), req)
					if err != nil || got.ErrorCode != tt.code || got.Success != (tt.code == "") {
						t.Fatalf("result=%+v err=%v", got, err)
					}
					if tt.code != "" {
						select {
						case <-messages:
							t.Fatal("invalid signature was sent")
						default:
						}
						continue
					}
					if message := <-messages; message != tt.want {
						t.Fatalf("message=%q want=%q", message, tt.want)
					}
					var logged struct {
						Message string `json:"message"`
					}
					if err := json.Unmarshal([]byte(got.RequestData), &logged); err != nil || logged.Message != tt.want {
						t.Fatalf("request log=%s err=%v", got.RequestData, err)
					}
				}
				if req.RenderedContent != body {
					t.Fatal("sender mutated the reusable rendered body")
				}
			})
		}
	}
}
