package worker

import (
	"errors"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender"
	"gorm.io/gorm"
)

func TestResolveDeliveryDependenciesFailsBeforeSenderForRequiredSignature(t *testing.T) {
	tests := []struct {
		name        string
		alias       string
		lookup      *stubSignatureLookup
		wantError   string
		wantLookups int
	}{
		{
			name:        "missing alias",
			lookup:      &stubSignatureLookup{},
			wantError:   "required signature is missing",
			wantLookups: 0,
		},
		{
			name:        "mapping removed",
			alias:       "default",
			lookup:      &stubSignatureLookup{err: gorm.ErrRecordNotFound},
			wantError:   "required signature alias cannot be resolved",
			wantLookups: 1,
		},
		{
			name:        "empty resolved mapping",
			alias:       "default",
			lookup:      &stubSignatureLookup{},
			wantError:   "required signature is missing",
			wantLookups: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			senders := &stubSenderLookup{}
			_, _, err := resolveDeliveryDependencies(
				tt.lookup,
				senders,
				&model.PushTask{ChannelID: 1, Signature: tt.alias},
				&model.ProviderAccount{ID: 2, ProviderCode: "required"},
				true,
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
			if tt.lookup.calls != tt.wantLookups {
				t.Fatalf("signature lookups = %d, want %d", tt.lookup.calls, tt.wantLookups)
			}
			if senders.calls != 0 {
				t.Fatalf("sender resolver was called %d times after required signature failure", senders.calls)
			}
		})
	}
}

func TestResolveDeliveryDependenciesAllowsOptionalSignatureMiss(t *testing.T) {
	signatures := &stubSignatureLookup{err: errors.New("not mapped")}
	senders := &stubSenderLookup{}
	providerSignature, _, err := resolveDeliveryDependencies(
		signatures,
		senders,
		&model.PushTask{ChannelID: 1, Signature: "optional"},
		&model.ProviderAccount{ID: 2, ProviderCode: "optional"},
		false,
	)
	if err != nil || providerSignature != nil {
		t.Fatalf("optional mapping miss = signature=%+v err=%v", providerSignature, err)
	}
	if senders.calls != 1 {
		t.Fatalf("optional path should resolve sender once, got %d", senders.calls)
	}
}

func TestSameNameTemplateParamsOnlyIncludesProviderVariables(t *testing.T) {
	tests := []struct {
		name              string
		templateParams    map[string]string
		providerVariables []string
		want              map[string]string
	}{
		{
			name: "provider variables are a system subset",
			templateParams: map[string]string{
				"code":     "123456",
				"name":     "Alice",
				"internal": "must-not-leak",
			},
			providerVariables: []string{"code", "name"},
			want:              map[string]string{"code": "123456", "name": "Alice"},
		},
		{
			name:              "provider template has no variables",
			templateParams:    map[string]string{"internal": "must-not-leak"},
			providerVariables: nil,
			want:              map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sameNameTemplateParams(tt.templateParams, tt.providerVariables)
			if len(got) != len(tt.want) {
				t.Fatalf("mapped params = %v, want %v", got, tt.want)
			}
			for key, wantValue := range tt.want {
				if got[key] != wantValue {
					t.Fatalf("mapped params[%q] = %q, want %q", key, got[key], wantValue)
				}
			}
			if _, leaked := got["internal"]; leaked {
				t.Fatalf("extra system variable leaked to provider: %v", got)
			}
		})
	}
}

type stubSignatureLookup struct {
	signature *model.ProviderSignature
	err       error
	calls     int
}

func (s *stubSignatureLookup) GetByChannelIDAndSignatureName(uint, string, uint) (*model.ProviderSignature, error) {
	s.calls++
	return s.signature, s.err
}

type stubSenderLookup struct {
	calls int
}

func (s *stubSenderLookup) GetSender(string) (sender.Sender, error) {
	s.calls++
	return nil, nil
}
