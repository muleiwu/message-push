package helper

import "testing"

func TestParsePhoneNumber(t *testing.T) {
	t.Run("domestic numbers parse to CN with bare national + E.164", func(t *testing.T) {
		for _, in := range []string{"13800138000", "+8613800138000", "008613800138000", "8613800138000"} {
			got := ParsePhoneNumber(in)
			if !got.Valid {
				t.Errorf("ParsePhoneNumber(%q).Valid = false, want true", in)
				continue
			}
			if got.Region != "CN" {
				t.Errorf("ParsePhoneNumber(%q).Region = %q, want CN", in, got.Region)
			}
			if got.CountryCode != "86" {
				t.Errorf("ParsePhoneNumber(%q).CountryCode = %q, want 86", in, got.CountryCode)
			}
			if got.NationalNumber != "13800138000" {
				t.Errorf("ParsePhoneNumber(%q).NationalNumber = %q, want 13800138000", in, got.NationalNumber)
			}
			if got.E164 != "+8613800138000" {
				t.Errorf("ParsePhoneNumber(%q).E164 = %q, want +8613800138000", in, got.E164)
			}
		}
	})

	t.Run("international number reports its own region", func(t *testing.T) {
		got := ParsePhoneNumber("+14155552671")
		if !got.Valid {
			t.Fatalf("ParsePhoneNumber(US).Valid = false, want true")
		}
		if got.Region != "US" {
			t.Errorf("Region = %q, want US", got.Region)
		}
		if got.CountryCode != "1" {
			t.Errorf("CountryCode = %q, want 1", got.CountryCode)
		}
		if got.E164 != "+14155552671" {
			t.Errorf("E164 = %q, want +14155552671", got.E164)
		}
	})

	t.Run("invalid / non-phone inputs are not valid", func(t *testing.T) {
		for _, in := range []string{"", "abc", "12345", "not-a-phone@example.com"} {
			if got := ParsePhoneNumber(in); got.Valid {
				t.Errorf("ParsePhoneNumber(%q).Valid = true, want false (%+v)", in, got)
			}
		}
	})
}
