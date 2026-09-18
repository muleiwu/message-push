package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
)

type aliyunTestHTTP func(*http.Request) (*http.Response, error)

func (f aliyunTestHTTP) Call(r *http.Request, _ *http.Transport) (*http.Response, error) { return f(r) }

func aliyunTestAccount() *model.ProviderAccount {
	return &model.ProviderAccount{ID: 1, ProviderCode: constants.ProviderAliyunSMS, ProviderType: "sms", Status: 1, Config: `{"access_key_id":"fixture-aliyun-id","access_key_secret":"fixture-aliyun-secret"}`}
}

func aliyunTestFactory(t *testing.T, handle func(*http.Request, string, url.Values) (map[string]any, error)) aliyunClientFactory {
	t.Helper()
	return func(account *model.ProviderAccount) (*dysmsapi.Client, error) {
		client, err := newAliyunClient(account)
		if err != nil {
			return nil, err
		}
		client.HttpClient = aliyunTestHTTP(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != aliyunSMSEndpoint || r.Method != http.MethodPost {
				t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL)
			}
			// Dara constructs lower-case header keys directly. Normalize as an HTTP
			// server would before using net/http's form parser in this transport double.
			headers := http.Header{}
			for key, values := range r.Header {
				for _, value := range values {
					headers.Add(key, value)
				}
			}
			r.Header = headers
			if r.Body == nil {
				r.Form = r.URL.Query()
			} else if err := r.ParseForm(); err != nil {
				return nil, err
			}
			action := r.Header.Get("x-acs-action")
			for name, values := range r.Header {
				if strings.EqualFold(name, "x-acs-action") && len(values) > 0 {
					action = values[0]
				}
			}
			if action == "" {
				action = r.Form.Get("Action")
			}
			body, err := handle(r, action, r.Form)
			if err != nil {
				return nil, err
			}
			if body["RequestId"] == nil {
				body["RequestId"] = "request-" + action
			}
			if body["Code"] == nil {
				body["Code"] = "OK"
			}
			data, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
		})
		return client, nil
	}
}

func aliyunTestInput(kind domain.ResourceKind) domain.ResourceInput {
	if kind == domain.ResourceTemplates {
		return domain.ResourceInput{ID: "SMS_123", Name: "登录验证码", Content: "验证码${code}", Category: "0", Description: "用于登录",
			ProviderFields: map[string]string{"related_sign_name": "木雷科技", "template_rule": `{"code":"characterWithNumber"}`, "apply_scene_content": "已上线的用户登录服务"}}
	}
	return domain.ResourceInput{ID: "木雷科技", Content: "木雷科技", Description: "用于登录", ProviderFields: map[string]string{"qualification_id": "9007199254740993", "sign_source": "0", "sign_type": "1"}}
}

func aliyunTestResponse(action string) map[string]any {
	switch action {
	case "GetSmsTemplate":
		return map[string]any{"TemplateCode": "SMS_123", "TemplateName": "登录验证码", "TemplateContent": "验证码${code}", "TemplateType": "0", "TemplateStatus": "2", "OrderId": "order-old", "RelatedSignName": "木雷科技", "VariableAttribute": `{"code":"characterWithNumber"}`, "ApplyScene": "登录", "Remark": "用于登录"}
	case "GetSmsSign":
		return map[string]any{"SignName": "木雷科技", "SignStatus": 1, "SignCode": "SIGN_metadata_only", "OrderId": "order-old", "QualificationId": int64(9007199254740993), "ThirdParty": false, "FileUrlList": []string{"https://private.example/image?token=secret"}, "SignIspRegisterDetailList": []any{map[string]any{"OperatorCode": "mobile", "RegisterStatus": 1}}}
	case "QuerySmsTemplateList":
		return map[string]any{"SmsTemplateList": []any{map[string]any{"TemplateCode": "SMS_123", "TemplateContent": "验证码${code}", "OuterTemplateType": 0, "AuditStatus": "AUDIT_STATE_NOT_PASS", "OrderId": "order-old"}}, "TotalCount": 1, "CurrentPage": 1}
	case "QuerySmsSignList":
		return map[string]any{"SmsSignList": []any{map[string]any{"SignName": "木雷科技", "AuditStatus": "AUDIT_STATE_PASS", "OrderId": "order-old"}}, "TotalCount": 1, "CurrentPage": 1}
	case "CreateSmsTemplate", "UpdateSmsTemplate":
		return map[string]any{"TemplateCode": "SMS_123", "OrderId": "order-new"}
	case "CreateSmsSign", "UpdateSmsSign":
		return map[string]any{"SignName": "木雷科技", "OrderId": "order-new"}
	case "DeleteSmsSign":
		return map[string]any{"SignName": "木雷科技"}
	case "DeleteSmsTemplate":
		return map[string]any{}
	}
	return map[string]any{"Code": "UnsupportedOperation"}
}

func TestAliyunResourceWireContracts(t *testing.T) {
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		for _, action := range []domain.ResourceAction{domain.ResourceQuery, domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete} {
			t.Run(string(kind)+"/"+string(action), func(t *testing.T) {
				calls := []string{}
				factory := aliyunTestFactory(t, func(_ *http.Request, api string, values url.Values) (map[string]any, error) {
					calls = append(calls, api)
					if strings.HasPrefix(api, "Query") && (values.Get("PageSize") != "50" || values.Get("PageIndex") != "1") {
						t.Errorf("wrong pagination: %v", values)
					}
					if strings.HasPrefix(api, "Create") || strings.HasPrefix(api, "Update") {
						if kind == domain.ResourceTemplates {
							if values.Get("TemplateContent") != "验证码${code}" || values.Get("TemplateType") != "0" || values.Get("RelatedSignName") != "木雷科技" || values.Get("TemplateRule") != `{"code":"characterWithNumber"}` {
								t.Errorf("wrong template fields: %v", values)
							}
							if values.Has("IntlType") {
								t.Error("international parameter sent")
							}
						} else if values.Get("SignName") != "木雷科技" || values.Get("QualificationId") != "9007199254740993" || values.Get("ThirdParty") != "false" || values.Get("SignType") != "1" {
							t.Errorf("wrong sign fields: %v", values)
						}
					}
					if strings.HasPrefix(api, "Get") || strings.HasPrefix(api, "Delete") {
						key, id := "TemplateCode", "SMS_123"
						if kind == domain.ResourceSignatures {
							key, id = "SignName", "木雷科技"
						}
						if values.Get(key) != id {
							t.Errorf("wrong identity: %v", values)
						}
					}
					return aliyunTestResponse(api), nil
				})
				definition := aliyunResourceDefinitions(factory)[kind]
				if err := definition.Validate(kind); err != nil {
					t.Fatal(err)
				}
				input := aliyunTestInput(kind)
				if action == domain.ResourceQuery || action == domain.ResourceCreate {
					input.ID = ""
				}
				rows, err := definition.Execute(context.Background(), aliyunTestAccount(), action, input)
				if err != nil || len(rows) != 1 {
					var remote *domain.RemoteResourceError
					if errors.As(err, &remote) {
						t.Logf("diagnostic: %+v", remote.Diagnostic)
					}
					t.Fatalf("execute: %+v %v", rows, err)
				}
				if rows[0].ID != aliyunTestInput(kind).ID || rows[0].RequestID == "" {
					t.Fatalf("invalid identity: %+v", rows)
				}
				expectedCalls := 1
				if action == domain.ResourceUpdate || action == domain.ResourceDelete {
					expectedCalls = 2
				}
				if len(calls) != expectedCalls {
					t.Fatalf("unexpected calls: %v", calls)
				}
			})
		}
	}
}

func TestAliyunDetailsUseNamesAndOmitMaterials(t *testing.T) {
	factory := aliyunTestFactory(t, func(_ *http.Request, api string, _ url.Values) (map[string]any, error) {
		return aliyunTestResponse(api), nil
	})
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		rows, err := aliyunResourceDefinitions(factory)[kind].Execute(context.Background(), aliyunTestAccount(), domain.ResourceQuery, domain.ResourceInput{ID: aliyunTestInput(kind).ID})
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(rows)
		if strings.Contains(string(encoded), "private.example") || strings.Contains(string(encoded), "FileUrl") {
			t.Fatal("material leaked into resource")
		}
		if kind == domain.ResourceSignatures {
			if rows[0].ID != "木雷科技" || rows[0].Content != "木雷科技" || rows[0].ProviderFields["qualification_id"] != "9007199254740993" {
				t.Fatalf("wrong signature fields: %+v", rows[0])
			}
			if rows[0].ProviderMetadata["aliyun"].(map[string]any)["isp_registration"] == nil {
				t.Fatal("missing registration details")
			}
		} else if rows[0].ProviderFields["template_rule"] != `{"code":"characterWithNumber"}` {
			t.Fatal("missing edit fields")
		}
	}
}

func TestAliyunPaginationAndDomesticTypes(t *testing.T) {
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		for _, broken := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/broken=%v", kind, broken), func(t *testing.T) {
				calls := 0
				factory := aliyunTestFactory(t, func(_ *http.Request, _ string, values url.Values) (map[string]any, error) {
					calls++
					page, _ := strconv.Atoi(values.Get("PageIndex"))
					if broken && page == 2 {
						return nil, errors.New("page failed")
					}
					list := []any{}
					for i := (page - 1) * 50; i < page*50 && i < 53; i++ {
						if kind == domain.ResourceTemplates {
							row := map[string]any{"TemplateCode": fmt.Sprint(i), "TemplateContent": "普通通知", "AuditStatus": "AUDIT_STATE_PASS"}
							if i < 50 {
								row["OuterTemplateType"] = 3
							} else {
								row["TemplateType"] = []int{2, 0, 1}[i-50]
							}
							list = append(list, row)
						} else {
							list = append(list, map[string]any{"SignName": fmt.Sprintf("签名%d", i), "AuditStatus": "AUDIT_STATE_PASS"})
						}
					}
					key := "SmsTemplateList"
					if kind == domain.ResourceSignatures {
						key = "SmsSignList"
					}
					return map[string]any{key: list, "TotalCount": 53, "CurrentPage": page}, nil
				})
				rows, err := aliyunResourceDefinitions(factory)[kind].Execute(context.Background(), aliyunTestAccount(), domain.ResourceQuery, domain.ResourceInput{})
				if calls != 2 {
					t.Fatalf("not all pages read: %d", calls)
				}
				if broken {
					if err == nil || rows != nil {
						t.Fatal("returned partial result")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if kind == domain.ResourceTemplates {
					if len(rows) != 3 || rows[0].Category != "0" || rows[1].Category != "1" || rows[2].Category != "2" {
						t.Fatalf("wrong domestic categories: %+v", rows)
					}
				} else if len(rows) != 53 {
					t.Fatalf("missing signatures: %d", len(rows))
				}
			})
		}
	}
}

func TestAliyunValidationAndOperationRestrictions(t *testing.T) {
	definitions := aliyunResourceDefinitions(nil)
	for _, test := range []struct {
		kind   domain.ResourceKind
		change func(*domain.ResourceInput)
	}{
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.Category = "3" }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { delete(i.ProviderFields, "template_rule") }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.ProviderFields["template_rule"] = `{"other":"text"}` }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.ProviderFields["template_rule"] = `{"code":1}` }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.ProviderFields["proof_image"] = "data" }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.Category = "2" }},
		{domain.ResourceTemplates, func(i *domain.ResourceInput) { i.ProviderFields["more_data"] = `["https://image.example/x?token=a"]` }},
		{domain.ResourceSignatures, func(i *domain.ResourceInput) { i.Content = "另一个签名" }},
		{domain.ResourceSignatures, func(i *domain.ResourceInput) { i.ProviderFields["qualification_id"] = "1e6" }},
		{domain.ResourceSignatures, func(i *domain.ResourceInput) { i.ProviderFields["third_party"] = "true" }},
		{domain.ResourceSignatures, func(i *domain.ResourceInput) { i.ProviderFields["sign_source"] = "5" }},
		{domain.ResourceSignatures, func(i *domain.ResourceInput) { i.ProviderFields["sign_source"] = "2" }},
	} {
		input := aliyunTestInput(test.kind)
		test.change(&input)
		if err := definitions[test.kind].ValidateInput(domain.ResourceUpdate, input); err == nil {
			t.Errorf("invalid input accepted: %+v", input)
		}
	}
	for raw, normalized := range map[string]int8{"0": 1, "1": 2, "2": 3, "10": 0, "AUDIT_STATE_INIT": 1, "AUDIT_STATE_PASS": 2, "AUDIT_STATE_NOT_PASS": 3, "AUDIT_STATE_CANCEL": 0, "AUDIT_SATE_CANCEL": 0, "": 0, "future": 0} {
		if aliyunAudit(raw) != normalized {
			t.Errorf("audit %s", raw)
		}
	}
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		for _, status := range []int8{0, 1, 2, 3} {
			row := domain.RemoteResource{ResourceInput: domain.ResourceInput{Category: "0"}, AuditStatus: status}
			errs := definitions[kind].OperationErrors(row)
			if (errs[domain.ResourceDelete] == "") != (status == 2 || status == 3) {
				t.Errorf("delete policy: %s %d", kind, status)
			}
			if (errs[domain.ResourceUpdate] == "") != (status == 3 || kind == domain.ResourceSignatures && status == 2) {
				t.Errorf("update policy: %s %d", kind, status)
			}
		}
	}
}

func TestAliyunNoRetryAndCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		calls := 0
		factory := aliyunTestFactory(t, func(r *http.Request, _ string, _ url.Values) (map[string]any, error) {
			calls++
			if canceled {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			return nil, errors.New("connection lost")
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		input := aliyunTestInput(domain.ResourceSignatures)
		input.ID = ""
		rows, err := aliyunResourceDefinitions(factory)[domain.ResourceSignatures].Execute(ctx, aliyunTestAccount(), domain.ResourceCreate, input)
		cancel()
		var remote *domain.RemoteResourceError
		if calls != 1 || rows != nil || !errors.As(err, &remote) || !remote.Uncertain {
			t.Fatalf("unsafe retry/failure: calls=%d rows=%v err=%v", calls, rows, err)
		}
	}
}

func TestAliyunErrorsRetainCodesAndRedactMaterialReferences(t *testing.T) {
	for _, code := range []string{"isp.SYSTEM_ERROR", "InternalError", "RequestTimeout", "ServiceUnavailable"} {
		if !aliyunUncertainCode(code) {
			t.Fatalf("ambiguous upstream failure classified as rejection: %s", code)
		}
	}
	for _, status := range []int{400, 500} {
		calls := 0
		factory := func(account *model.ProviderAccount) (*dysmsapi.Client, error) {
			client, err := newAliyunClient(account)
			if err != nil {
				return nil, err
			}
			client.HttpClient = aliyunTestHTTP(func(*http.Request) (*http.Response, error) {
				calls++
				data := `{"Code":"InvalidMoreData","Message":"bad private/proof.png fixture-aliyun-secret","RequestId":"error-request"}`
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(data))}, nil
			})
			return client, nil
		}
		input := aliyunTestInput(domain.ResourceSignatures)
		input.ID = ""
		input.ProviderFields["more_data"] = `["private/proof.png"]`
		_, err := aliyunResourceDefinitions(factory)[domain.ResourceSignatures].Execute(context.Background(), aliyunTestAccount(), domain.ResourceCreate, input)
		var remote *domain.RemoteResourceError
		if !errors.As(err, &remote) || remote.Code != "InvalidMoreData" || remote.RequestID != "error-request" || remote.Uncertain != (status == 500) || calls != 1 {
			t.Fatalf("error contract: %+v calls=%d err=%v", remote, calls, err)
		}
		text := remote.Message + remote.Diagnostic.Cause + remote.Diagnostic.ResponseBody
		if strings.Contains(text, "private/proof.png") || strings.Contains(text, "fixture-aliyun-secret") {
			t.Fatal("submission material or credential leaked")
		}
	}
}

func TestAliyunRejectsInvalidRemoteIdentityAndInternationalMutation(t *testing.T) {
	for _, scenario := range []string{"missing_order", "changed_identity", "international", "pending", "unknown_type", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			writes := 0
			factory := aliyunTestFactory(t, func(_ *http.Request, api string, _ url.Values) (map[string]any, error) {
				body := aliyunTestResponse(api)
				if api == "UpdateSmsTemplate" {
					writes++
					if scenario == "missing_order" {
						delete(body, "OrderId")
					}
					if scenario == "changed_identity" {
						body["TemplateCode"] = "SMS_other"
					}
				}
				if api == "GetSmsTemplate" {
					if scenario == "international" {
						body["TemplateType"] = "3"
					}
					if scenario == "pending" {
						body["TemplateStatus"] = "0"
					}
				}
				if api == "QuerySmsTemplateList" {
					list := body["SmsTemplateList"].([]any)
					if scenario == "unknown_type" {
						list[0].(map[string]any)["OuterTemplateType"] = 9
					}
					if scenario == "duplicate" {
						body["SmsTemplateList"] = append(list, list[0])
						body["TotalCount"] = 2
					}
				}
				return body, nil
			})
			action := domain.ResourceUpdate
			input := aliyunTestInput(domain.ResourceTemplates)
			if scenario == "unknown_type" || scenario == "duplicate" {
				action = domain.ResourceQuery
				input.ID = ""
			}
			rows, err := aliyunResourceDefinitions(factory)[domain.ResourceTemplates].Execute(context.Background(), aliyunTestAccount(), action, input)
			if err == nil || rows != nil {
				t.Fatal("invalid remote response accepted")
			}
			if scenario == "missing_order" || scenario == "changed_identity" {
				var remote *domain.RemoteResourceError
				if writes != 1 || !errors.As(err, &remote) || !remote.Uncertain {
					t.Fatal("invalid acknowledgement was treated as a definite failure")
				}
			} else if writes != 0 {
				t.Fatal("out-of-scope resource was mutated")
			}
		})
	}
}

func TestAliyunThirdPartyTrademarkAndMarketingMaterials(t *testing.T) {
	factory := aliyunTestFactory(t, func(_ *http.Request, api string, values url.Values) (map[string]any, error) {
		if values.Get("MoreData") != `["private/proof.png"]` {
			t.Errorf("material serialization: %v", values)
		}
		if api == "CreateSmsSign" {
			if values.Get("SignSource") != "5" || values.Get("ThirdParty") != "true" || values.Get("AuthorizationLetterId") != "12345" || values.Get("TrademarkId") != "56789" {
				t.Errorf("signature fields: %v", values)
			}
		} else if values.Get("TemplateType") != "2" || values.Get("TrafficDriving") == "" {
			t.Errorf("marketing fields: %v", values)
		}
		return aliyunTestResponse(api), nil
	})
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		input := aliyunTestInput(kind)
		input.ID = ""
		input.ProviderFields["more_data"] = `["private/proof.png"]`
		if kind == domain.ResourceSignatures {
			input.ProviderFields["sign_source"], input.ProviderFields["third_party"] = "5", "true"
			input.ProviderFields["authorization_letter_id"], input.ProviderFields["trademark_id"] = "12345", "56789"
		} else {
			input.Category = "2"
			input.ProviderFields["traffic_driving"] = `[{"trafficDrivingType":"DOMAIN","trafficDrivingContent":"example.com"}]`
		}
		if _, err := aliyunResourceDefinitions(factory)[kind].Execute(context.Background(), aliyunTestAccount(), domain.ResourceCreate, input); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAliyunSendingUsesNativeTemplateAndSignName(t *testing.T) {
	calls := []string{}
	factory := aliyunTestFactory(t, func(_ *http.Request, api string, values url.Values) (map[string]any, error) {
		calls = append(calls, api)
		switch api {
		case "SendSms":
			if values.Get("SignName") != "木雷科技" || values.Get("TemplateCode") != "SMS_123" || values.Get("TemplateParam") != `{"code":"123456"}` {
				t.Errorf("incorrect single send: %v", values)
			}
			return map[string]any{"BizId": "biz"}, nil
		case "SendBatchSms":
			if values.Get("SignNameJson") != `["木雷科技","木雷科技"]` || values.Get("TemplateCode") != "SMS_123" {
				t.Errorf("incorrect batch send: %v", values)
			}
			return map[string]any{"BizId": "batch"}, nil
		case "QuerySendDetails":
			return map[string]any{"SmsSendDetailDTOs": map[string]any{"SmsSendDetailDTO": []any{map[string]any{"PhoneNum": "13800138000", "SendStatus": 3, "ErrCode": "DELIVRD"}}}}, nil
		}
		return nil, fmt.Errorf("unexpected %s", api)
	})
	account := aliyunTestAccount()
	sender := &AliyunSMSSender{clientFactory: factory}
	binding := &model.ChannelTemplateBinding{ProviderID: account.ID, Status: 1, IsActive: 1, ProviderTemplate: &model.ProviderTemplate{ProviderID: account.ID, TemplateCode: "SMS_123", TemplateContent: "验证码${code}", ContentVersion: 1, ProviderAccount: account, Status: 1}, MappedContentVersion: 1}
	sign := &model.ProviderSignature{SignatureCode: "木雷科技", RemoteID: "木雷科技"}
	task := &model.PushTask{TaskID: "task", Receiver: "13800138000"}
	result, err := sender.Send(context.Background(), &domain.SendRequest{ProviderAccount: account, ChannelTemplateBinding: binding, Signature: sign, Task: task, MappedParams: map[string]string{"code": "123456"}})
	if err != nil || !result.Success || result.ProviderID != "biz" {
		t.Fatalf("send: %+v %v", result, err)
	}
	batch, err := sender.BatchSend(context.Background(), &domain.BatchSendRequest{ProviderAccount: account, ChannelTemplateBinding: binding, Signature: sign, Tasks: []*model.PushTask{task, task}, MappedParams: map[string]string{"code": "123456"}})
	if err != nil || len(batch.Results) != 2 || !batch.Results[0].Success || batch.Results[1].ProviderID != "batch_1" {
		t.Fatalf("batch: %+v %v", batch, err)
	}
	query, err := sender.QueryStatus(context.Background(), &domain.StatusQueryRequest{ProviderAccount: account, PhoneNumber: task.Receiver})
	if err != nil || len(query.Results) != 1 || query.Results[0].Status != constants.CallbackStatusDelivered {
		t.Fatalf("status: %+v %v", query, err)
	}
	if !reflect.DeepEqual(calls, []string{"SendSms", "SendBatchSms", "QuerySendDetails"}) {
		t.Fatalf("extra calls: %v", calls)
	}
}
