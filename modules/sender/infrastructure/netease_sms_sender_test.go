package infrastructure

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
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
		if results[1].Status != constants.CallbackStatusFailed || results[1].ErrorMessage != "空号" {
			t.Errorf("result[1] = %+v, want failed with reason", results[1])
		}
	})

	t.Run("uplink message ignored", func(t *testing.T) {
		body := `{"eventType":"12","objects":[{"mobile":"13800000000","sendid":"1"}]}`
		_, results, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte(body)})
		if err != nil {
			t.Fatalf("HandleCallback error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("uplink results len = %d, want 0", len(results))
		}
	})

	t.Run("invalid body returns error", func(t *testing.T) {
		_, _, err := s.HandleCallback(context.Background(), &domain.CallbackRequest{RawBody: []byte("not json")})
		if err == nil {
			t.Error("expected error for invalid body, got nil")
		}
	})
}
