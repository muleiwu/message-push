package infrastructure

import (
	"testing"
)

func TestZrwinfoNormalizeCNMobile(t *testing.T) {
	// 直接构造，避免 NewZrwinfoSMSSender() 初始化 DAO 依赖数据库
	s := &ZrwinfoSMSSender{}

	t.Run("valid domestic formats normalize to bare 11 digits", func(t *testing.T) {
		cases := []string{
			"13800138000",      // 纯 11 位
			" 13800138000 ",    // 带空白
			"+8613800138000",   // E.164
			"008613800138000",  // 00 前缀
			"8613800138000",    // 无 + 国家码
		}
		for _, in := range cases {
			got, err := s.normalizeCNMobile(in)
			if err != nil {
				t.Errorf("normalizeCNMobile(%q) unexpected error: %v", in, err)
				continue
			}
			if got != "13800138000" {
				t.Errorf("normalizeCNMobile(%q) = %q, want %q", in, got, "13800138000")
			}
		}
	})

	t.Run("non-domestic or invalid numbers return error", func(t *testing.T) {
		cases := []string{
			"",                // 空串
			"+14155552671",    // 美国号
			"+447911123456",   // 英国号
			"12345",           // 太短
			"19900000000000",  // 非法
			"abcdefg",         // 非数字
		}
		for _, in := range cases {
			if got, err := s.normalizeCNMobile(in); err == nil {
				t.Errorf("normalizeCNMobile(%q) = %q, want error", in, got)
			}
		}
	})
}
