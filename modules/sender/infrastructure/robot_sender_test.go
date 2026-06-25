package infrastructure

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSignDingTalkRobot(t *testing.T) {
	const secret = "SECtestsecret1234567890"
	const ts int64 = 1700000000000

	got := signDingTalkRobot(ts, secret)

	// 独立复算一遍期望值，确保算法为 base64(HMAC-SHA256(secret, ts+"\n"+secret))
	stringToSign := fmt.Sprintf("%d\n%s", ts, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	want := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if got != want {
		t.Fatalf("sign mismatch: got %q want %q", got, want)
	}
}

func TestParseWeChatRobotMentions(t *testing.T) {
	cases := []struct {
		name           string
		receiver       string
		wantList       []string
		wantMobileList []string
	}{
		{"empty", "", nil, nil},
		{"all", "@all", []string{"@all"}, nil},
		{"all_lower", "all", []string{"@all"}, nil},
		{"mobile", "13800001111", nil, []string{"13800001111"}},
		{"userid", "wangqing", []string{"wangqing"}, nil},
		{"mixed", "wangqing, 13800001111 ,@all", []string{"wangqing", "@all"}, []string{"13800001111"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			list, mobile := parseWeChatRobotMentions(c.receiver)
			if !reflect.DeepEqual(list, c.wantList) {
				t.Errorf("list: got %v want %v", list, c.wantList)
			}
			if !reflect.DeepEqual(mobile, c.wantMobileList) {
				t.Errorf("mobile: got %v want %v", mobile, c.wantMobileList)
			}
		})
	}
}

func TestParseDingTalkRobotAt(t *testing.T) {
	cases := []struct {
		name        string
		receiver    string
		wantMobiles []string
		wantUserIds []string
		wantIsAtAll bool
	}{
		{"empty", "", nil, nil, false},
		{"all", "@all", nil, nil, true},
		{"mobile", "13800001111", []string{"13800001111"}, nil, false},
		{"userid", "manager123", nil, []string{"manager123"}, false},
		{"mixed", "manager123,13800001111,all", []string{"13800001111"}, []string{"manager123"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mobiles, userIds, isAtAll := parseDingTalkRobotAt(c.receiver)
			if !reflect.DeepEqual(mobiles, c.wantMobiles) {
				t.Errorf("mobiles: got %v want %v", mobiles, c.wantMobiles)
			}
			if !reflect.DeepEqual(userIds, c.wantUserIds) {
				t.Errorf("userIds: got %v want %v", userIds, c.wantUserIds)
			}
			if isAtAll != c.wantIsAtAll {
				t.Errorf("isAtAll: got %v want %v", isAtAll, c.wantIsAtAll)
			}
		})
	}
}

func TestParseWeChatAgentID(t *testing.T) {
	okCases := []struct {
		in   interface{}
		want int
	}{
		{"1000002", 1000002},
		{" 1000002 ", 1000002},
		{float64(1000002), 1000002}, // JSON 数字反序列化为 float64
		{int(7), 7},
	}
	for _, c := range okCases {
		got, err := parseWeChatAgentID(c.in)
		if err != nil {
			t.Errorf("parseWeChatAgentID(%v) unexpected err: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseWeChatAgentID(%v) = %d want %d", c.in, got, c.want)
		}
	}

	errCases := []interface{}{"", "  ", "abc", nil}
	for _, c := range errCases {
		if _, err := parseWeChatAgentID(c); err == nil {
			t.Errorf("parseWeChatAgentID(%v) expected error, got nil", c)
		}
	}
}

func TestTruncateUTF8Bytes(t *testing.T) {
	// 短于上限：原样返回
	if got := truncateUTF8Bytes("hello", 2048); got != "hello" {
		t.Errorf("short string changed: %q", got)
	}
	// 恰好等于上限：原样返回
	if got := truncateUTF8Bytes("abcd", 4); got != "abcd" {
		t.Errorf("exact-limit string changed: %q", got)
	}
	// ASCII 超限：按字节截断
	if got := truncateUTF8Bytes("abcdef", 4); got != "abcd" {
		t.Errorf("ascii truncate = %q want %q", got, "abcd")
	}
	// 多字节不被切断：每个中文 3 字节，限 7 字节应只保留 2 个字（6 字节）
	s := "中文测试"
	got := truncateUTF8Bytes(s, 7)
	if got != "中文" {
		t.Errorf("utf8 truncate = %q want %q", got, "中文")
	}
	if len(got) > 7 {
		t.Errorf("truncated len %d exceeds limit 7", len(got))
	}
	if strings.ContainsRune(got, '�') {
		t.Errorf("truncation produced invalid rune: %q", got)
	}
}
