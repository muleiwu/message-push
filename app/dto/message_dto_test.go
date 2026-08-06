package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
)

func TestScheduledAtAcceptsEquivalentRFC3339Offsets(t *testing.T) {
	decode := func(value string) time.Time {
		t.Helper()
		var request SendRequest
		if err := json.Unmarshal([]byte(`{"scheduled_at":"`+value+`"}`), &request); err != nil {
			t.Fatalf("decode scheduled_at %q: %v", value, err)
		}
		if request.ScheduledAt == nil {
			t.Fatalf("scheduled_at %q decoded as nil", value)
		}
		return *request.ScheduledAt
	}

	utc := decode("2026-08-04T10:00:00Z")
	shanghai := decode("2026-08-04T18:00:00+08:00")
	if !utc.Equal(shanghai) {
		t.Fatalf("equivalent scheduled_at values differ: %s vs %s", utc, shanghai)
	}
}

func TestMessageSignatureAliasRejectsMoreThan100Characters(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")

	request := SendRequest{
		ChannelID:     1,
		Receiver:      "receiver@example.com",
		SignatureName: strings.Repeat("a", 101),
	}
	if err := validate.Struct(request); err == nil {
		t.Fatal("expected a 101-character signature alias to be rejected")
	}

	request.SignatureName = strings.Repeat("a", 100)
	if err := validate.Struct(request); err != nil {
		t.Fatalf("100-character signature alias was rejected: %v", err)
	}
}

func TestChannelMappingAliasRejectsMoreThan100Characters(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")

	request := CreateChannelSignatureMappingRequest{
		SignatureName:       strings.Repeat("a", 101),
		ProviderSignatureID: 1,
		ProviderID:          1,
	}
	if err := validate.Struct(request); err == nil {
		t.Fatal("expected a 101-character channel mapping alias to be rejected")
	}

	request.SignatureName = strings.Repeat("a", 100)
	if err := validate.Struct(request); err != nil {
		t.Fatalf("100-character channel mapping alias was rejected: %v", err)
	}
}
