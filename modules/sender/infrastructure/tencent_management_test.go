package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type tencentRoundTrip func(*http.Request) (*http.Response, error)

func (f tencentRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func tencentTestAccount() *model.ProviderAccount {
	return &model.ProviderAccount{ID: 1, ProviderCode: constants.ProviderTencentSMS, ProviderType: "sms", Status: 1, Config: `{"secret_id":"fixture-secret-id","secret_key":"fixture-secret-key","sdk_app_id":"1400000001"}`}
}

func tencentTestFactory(t *testing.T, handle func(*http.Request, string, map[string]any) (map[string]any, error)) tencentClientFactory {
	t.Helper()
	return func(account *model.ProviderAccount) (*sms.Client, error) {
		client, err := newTencentClient(account)
		if err != nil {
			return nil, err
		}
		client.WithHttpTransport(tencentRoundTrip(func(request *http.Request) (*http.Response, error) {
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				return nil, err
			}
			action := request.Header.Get("X-TC-Action")
			response, err := handle(request, action, body)
			if err != nil {
				return nil, err
			}
			if response["RequestId"] == nil {
				response["RequestId"] = "request-" + action
			}
			data, err := json.Marshal(map[string]any{"Response": response})
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
		}))
		return client, nil
	}
}

func tencentTestProof(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func tencentTestResourceResponse(action string) map[string]any {
	switch action {
	case "DescribeSmsTemplateList":
		return map[string]any{"DescribeTemplateStatusSet": []any{map[string]any{"TemplateId": 101, "TemplateName": "验证码", "TemplateContent": "验证码{1}", "International": 0, "StatusCode": 0}}}
	case "DescribeSmsSignList":
		return map[string]any{"DescribeSignListStatusSet": []any{map[string]any{"SignId": 101, "SignName": "测试签名", "International": 0, "StatusCode": 2, "QualificationId": 999, "QualificationName": "测试资质", "QualificationStatusCode": 1}}}
	case "AddSmsTemplate":
		return map[string]any{"AddTemplateStatus": map[string]any{"TemplateId": "101"}}
	case "ModifySmsTemplate":
		return map[string]any{"ModifyTemplateStatus": map[string]any{"TemplateId": 101}}
	case "DeleteSmsTemplate":
		return map[string]any{"DeleteTemplateStatus": map[string]any{"DeleteStatus": "return successfully!"}}
	case "AddSmsSign":
		return map[string]any{"AddSignStatus": map[string]any{"SignId": 101}}
	case "ModifySmsSign":
		return map[string]any{"ModifySignStatus": map[string]any{"SignId": 101}}
	case "DeleteSmsSign":
		return map[string]any{"DeleteSignStatus": map[string]any{"DeleteStatus": "return successfully!"}}
	default:
		return map[string]any{"Error": map[string]any{"Code": "UnsupportedOperation", "Message": "fixture action not implemented"}}
	}
}

func TestTencentResourceOperationsUseDomesticSDKRequests(t *testing.T) {
	proof := tencentTestProof(t)
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		for _, action := range []domain.ResourceAction{domain.ResourceQuery, domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete} {
			t.Run(string(kind)+"/"+string(action), func(t *testing.T) {
				calls := []string{}
				factory := tencentTestFactory(t, func(_ *http.Request, method string, body map[string]any) (map[string]any, error) {
					calls = append(calls, method)
					if !strings.HasPrefix(method, "Delete") && body["International"] != float64(0) {
						t.Errorf("not a domestic request: %v", body)
					}
					if strings.HasPrefix(method, "Describe") {
						if body["Limit"] != float64(100) || body["Offset"] != float64(0) {
							t.Errorf("pagination not explicit: %v", body)
						}
					} else if strings.HasPrefix(method, "Add") || strings.HasPrefix(method, "Modify") {
						if kind == domain.ResourceTemplates {
							if body["SmsType"] != float64(3) || body["TemplateContent"] != "验证码{1}" || body["Remark"] != "登录验证码" {
								t.Errorf("wrong template request: %v", body)
							}
						} else if body["SignType"] != float64(0) || body["DocumentType"] != float64(1) || body["SignPurpose"] != float64(1) || body["QualificationId"] != float64(999) || body["ProofImage"] != proof || body["CommissionImage"] != proof {
							t.Error("signature fields were not correctly serialized")
						}
					}
					return tencentTestResourceResponse(method), nil
				})
				definition := tencentResourceDefinitions(factory)[kind]
				if err := definition.Validate(kind); err != nil {
					t.Fatal(err)
				}
				input := domain.ResourceInput{ID: "101", Name: "验证码", Content: "验证码{1}", Category: "3", Description: "登录验证码"}
				if action == domain.ResourceCreate {
					input.ID = ""
				}
				if kind == domain.ResourceSignatures {
					input.Content = "测试签名"
					input.ProviderFields = map[string]string{"sign_type": "0", "document_type": "1", "sign_purpose": "1", "qualification_id": "999", "proof_image": proof, "commission_image": proof}
				}
				rows, err := definition.Execute(context.Background(), tencentTestAccount(), action, input)
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 1 || rows[0].ID != "101" || rows[0].RequestID == "" {
					t.Fatalf("invalid result: %+v", rows)
				}
				if action == domain.ResourceQuery && kind == domain.ResourceSignatures && (rows[0].AuditStatus != 1 || rows[0].ProviderMetadata["status_code"] != int64(2)) {
					t.Fatalf("pending activation treated as usable: %+v", rows)
				}
				if action == domain.ResourceQuery && kind == domain.ResourceTemplates && rows[0].Category != "" {
					t.Fatal("query invented an SMS type")
				}
				want := 1
				if action == domain.ResourceUpdate || action == domain.ResourceDelete {
					want = 2
				}
				if len(calls) != want {
					t.Fatalf("calls = %v", calls)
				}
			})
		}
	}
}

func TestTencentResourcePagingAndFailureNeverReturnPartialList(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		factory := tencentTestFactory(t, func(_ *http.Request, action string, body map[string]any) (map[string]any, error) {
			offset := int(body["Offset"].(float64))
			if offset == 100 && failSecond {
				return nil, errors.New("connection lost")
			}
			count := 100
			if offset == 100 {
				count = 1
			}
			rows := make([]any, count)
			for i := range rows {
				rows[i] = map[string]any{"SignId": offset + i + 1, "SignName": "签名", "International": 0, "StatusCode": 0}
			}
			return map[string]any{"DescribeSignListStatusSet": rows}, nil
		})
		rows, err := tencentResourceDefinitions(factory)[domain.ResourceSignatures].Execute(context.Background(), tencentTestAccount(), domain.ResourceQuery, domain.ResourceInput{})
		if failSecond {
			if err == nil || rows != nil {
				t.Fatal("partial list escaped after failure")
			}
		} else if err != nil || len(rows) != 101 {
			t.Fatalf("pagination: len=%d err=%v", len(rows), err)
		}
	}
}

func TestTencentResourceValidationPrecedesCloudCallsAndKeepsMaterialsPrivate(t *testing.T) {
	proof := tencentTestProof(t)
	calls := 0
	factory := tencentTestFactory(t, func(_ *http.Request, action string, body map[string]any) (map[string]any, error) {
		calls++
		return tencentTestResourceResponse(action), nil
	})
	definition := tencentResourceDefinitions(factory)[domain.ResourceSignatures]
	base := domain.ResourceInput{Content: "测试", ProviderFields: map[string]string{"sign_type": "0", "document_type": "1", "sign_purpose": "1", "qualification_id": "999", "proof_image": proof, "commission_image": proof}}
	for key, value := range map[string]string{"sign_type": "1", "document_type": "7", "qualification_id": "", "commission_image": "", "proof_image": "data:image/png;base64," + proof} {
		input := base
		input.ProviderFields = map[string]string{}
		for k, v := range base.ProviderFields {
			input.ProviderFields[k] = v
		}
		input.ProviderFields[key] = value
		if _, err := definition.Execute(context.Background(), tencentTestAccount(), domain.ResourceCreate, input); err == nil {
			t.Errorf("accepted invalid %s", key)
		}
	}
	if calls != 0 {
		t.Fatal("invalid material reached Tencent")
	}
	public := base.PublicCopy(definition.Operations[domain.ResourceCreate].Fields)
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), proof) || public.ProviderFields["qualification_id"] != "999" {
		t.Fatal("submission material leaked or public facts lost")
	}
	for _, code := range []int64{-1, 0, 1, 2, 99} {
		want := map[int64]int8{-1: 3, 0: 2, 1: 1, 2: 1, 99: 0}[code]
		if got := tencentAudit(&code); got != want {
			t.Fatalf("audit %d = %d", code, got)
		}
	}
	if tencentAudit(nil) != 0 {
		t.Fatal("missing audit status considered approved")
	}
}

func TestTencentWritesAreNotRetriedAndErrorsAreRedacted(t *testing.T) {
	calls := 0
	factory := tencentTestFactory(t, func(_ *http.Request, action string, body map[string]any) (map[string]any, error) {
		calls++
		return nil, errors.New("connection reset")
	})
	_, err := tencentResourceDefinitions(factory)[domain.ResourceTemplates].Execute(context.Background(), tencentTestAccount(), domain.ResourceCreate, domain.ResourceInput{Name: "验证码", Content: "验证码{1}", Category: "3", Description: "申请"})
	var remote *domain.RemoteResourceError
	if calls != 1 || !errors.As(err, &remote) || !remote.Uncertain {
		t.Fatalf("write retry/uncertainty: calls=%d err=%v", calls, err)
	}
	proof := tencentTestProof(t)
	factory = tencentTestFactory(t, func(_ *http.Request, _ string, _ map[string]any) (map[string]any, error) {
		return map[string]any{"Error": map[string]any{"Code": "InvalidParameterValue.ProofImage", "Message": proof + " fixture-secret-key"}}, nil
	})
	_, err = tencentResourceDefinitions(factory)[domain.ResourceSignatures].Execute(context.Background(), tencentTestAccount(), domain.ResourceCreate, domain.ResourceInput{Content: "测试", ProviderFields: map[string]string{"sign_type": "0", "document_type": "1", "sign_purpose": "0", "qualification_id": "999", "proof_image": proof}})
	if !errors.As(err, &remote) || remote.Uncertain || remote.RequestID == "" || strings.Contains(err.Error(), proof) || strings.Contains(err.Error(), "fixture-secret-key") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestTencentDomesticDeleteRejectsForeignResource(t *testing.T) {
	calls := 0
	factory := tencentTestFactory(t, func(_ *http.Request, action string, _ map[string]any) (map[string]any, error) {
		calls++
		return map[string]any{"DescribeSignListStatusSet": []any{map[string]any{"SignId": 101, "SignName": "foreign", "International": 1, "StatusCode": 0}}}, nil
	})
	_, err := tencentResourceDefinitions(factory)[domain.ResourceSignatures].Execute(context.Background(), tencentTestAccount(), domain.ResourceDelete, domain.ResourceInput{ID: "101"})
	if err == nil || calls != 1 {
		t.Fatal("foreign resource deletion was attempted")
	}
}

func TestTencentAllEventAPIsNormalizeFactsAndRespectLimits(t *testing.T) {
	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	for _, query := range []bool{false, true} {
		for _, kind := range []string{domain.SMSReports, domain.SMSReplies} {
			t.Run(fmt.Sprintf("query=%t/%s", query, kind), func(t *testing.T) {
				calls := 0
				factory := tencentTestFactory(t, func(_ *http.Request, action string, body map[string]any) (map[string]any, error) {
					calls++
					if body["SmsSdkAppId"] != "1400000001" || body["Limit"] != float64(2) {
						t.Errorf("wrong event request: %v", body)
					}
					if query && (body["PhoneNumber"] != "+8613800138000" || body["Offset"] != float64(0) || body["BeginTime"] != float64(now.Add(-time.Hour).Unix())) {
						t.Errorf("wrong historical query: %v", body)
					}
					if kind == domain.SMSReports {
						return map[string]any{"PullSmsSendStatusSet": []any{map[string]any{"SerialNo": "sid", "PhoneNumber": "+8613800138000", "ReportStatus": "SUCCESS", "UserReceiveTime": now.Unix()}, map[string]any{"SerialNo": "other", "SubscriberNumber": "13800138000", "CountryCode": "86", "ReportStatus": "PENDING"}}}, nil
					}
					return map[string]any{"PullSmsReplyStatusSet": []any{map[string]any{"PhoneNumber": "+8613800138000", "ReplyContent": "STOP", "SignName": "测试", "ExtendCode": "01", "ReplyTime": now.Unix()}}}, nil
				})
				s := &TencentSMSSender{clientFactory: factory, now: func() time.Time { return now }}
				req := &domain.SMSEventRequest{Account: tencentTestAccount(), Kind: kind, PhoneNumber: "13800138000", BeginTime: now.Add(-time.Hour), EndTime: now, Limit: 2}
				var response *domain.SMSEventResponse
				var err error
				if query {
					response, err = s.QuerySMSEvents(context.Background(), req)
				} else {
					response, err = s.PullSMSEvents(context.Background(), req)
				}
				if err != nil || calls != 1 || response.RequestID == "" {
					t.Fatalf("response=%+v err=%v calls=%d", response, err, calls)
				}
				if response.Items[0].Mobile != "+8613800138000" || response.Items[0].RawData == "" {
					t.Fatal("facts missing")
				}
				if kind == domain.SMSReports && (!response.LimitReached || response.Items[0].Status != "delivered" || response.Items[1].Status != "unknown") {
					t.Fatalf("status mapping: %+v", response)
				}
				if kind == domain.SMSReplies && (response.Items[0].Content != "STOP" || response.Items[0].SignName != "测试" || response.Items[0].ExtendCode != "01") {
					t.Fatalf("reply mapping: %+v", response)
				}
			})
		}
	}
}

func TestTencentEventCancellationRangeAndUncertainConsumption(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	calls := 0
	s := &TencentSMSSender{now: func() time.Time { return now }, clientFactory: tencentTestFactory(t, func(r *http.Request, _ string, _ map[string]any) (map[string]any, error) {
		calls++
		return nil, errors.New("network lost")
	})}
	req := &domain.SMSEventRequest{Account: tencentTestAccount(), Kind: domain.SMSReports, PhoneNumber: "13800138000", BeginTime: now.Add(-8 * 24 * time.Hour), EndTime: now, Limit: 100}
	if _, err := s.QuerySMSEvents(context.Background(), req); err == nil || calls != 0 {
		t.Fatal("invalid range reached cloud")
	}
	req.BeginTime = now.Add(-time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.QuerySMSEvents(ctx, req); err == nil || calls != 0 {
		t.Fatal("canceled request reached cloud")
	}
	_, err := s.PullSMSEvents(context.Background(), req)
	var remote *domain.RemoteResourceError
	if calls != 1 || !errors.As(err, &remote) || !remote.Uncertain {
		t.Fatalf("unsafe queue retry: %v calls=%d", err, calls)
	}
}

func TestTencentExistingSendUsesSDKAndNativeParameterOrder(t *testing.T) {
	account, binding := confirmedSMSBinding(constants.ProviderTencentSMS, "变量{2}，验证码{1}")
	account.Config = tencentTestAccount().Config
	calls := 0
	factory := tencentTestFactory(t, func(_ *http.Request, action string, body map[string]any) (map[string]any, error) {
		calls++
		if action != "SendSms" || !reflect.DeepEqual(body["TemplateParamSet"], []any{"1234", "5"}) {
			t.Errorf("wrong send wire: %s %v", action, body)
		}
		return map[string]any{"SendStatusSet": []any{map[string]any{"SerialNo": "sid", "PhoneNumber": "+8613800138000", "Code": "Ok", "Message": "send success"}}}, nil
	})
	s := &TencentSMSSender{clientFactory: factory}
	response, err := s.Send(context.Background(), &domain.SendRequest{Task: &model.PushTask{TaskID: "send", Receiver: "+8613800138000"}, ProviderAccount: account, ChannelTemplateBinding: binding, Signature: &model.ProviderSignature{SignatureCode: "测试"}, MappedParams: map[string]string{"1": "1234", "2": "5"}})
	if err != nil || !response.Success || calls != 1 {
		t.Fatalf("send: %+v %v calls=%d", response, err, calls)
	}
}
