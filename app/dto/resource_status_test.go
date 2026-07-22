package dto

import (
	"encoding/json"
	"testing"
)

func TestUpdateStatusDistinguishesDisabledFromOmitted(t *testing.T) {
	var disabled UpdateApplicationRequest
	if err := json.Unmarshal([]byte(`{"status":0}`), &disabled); err != nil {
		t.Fatalf("decode disabled status: %v", err)
	}
	if disabled.Status == nil || *disabled.Status != 0 {
		t.Fatalf("explicit status=0 was not preserved: %#v", disabled.Status)
	}

	var omitted UpdateApplicationRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("decode omitted status: %v", err)
	}
	if omitted.Status != nil {
		t.Fatalf("omitted status should remain nil: %#v", omitted.Status)
	}
}

func TestBindingUpdatePreservesZeroAndFalseValues(t *testing.T) {
	var req UpdateChannelBindingRequest
	if err := json.Unmarshal([]byte(`{"priority":0,"status":0,"is_active":0,"auto_disable_on_fail":false}`), &req); err != nil {
		t.Fatalf("decode binding update: %v", err)
	}
	if req.Priority == nil || *req.Priority != 0 {
		t.Fatalf("priority=0 was not preserved: %#v", req.Priority)
	}
	if req.Status == nil || *req.Status != 0 {
		t.Fatalf("status=0 was not preserved: %#v", req.Status)
	}
	if req.IsActive == nil || *req.IsActive != 0 {
		t.Fatalf("is_active=0 was not preserved: %#v", req.IsActive)
	}
	if req.AutoDisableOnFail == nil || *req.AutoDisableOnFail {
		t.Fatalf("auto_disable_on_fail=false was not preserved: %#v", req.AutoDisableOnFail)
	}
}
