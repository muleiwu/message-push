package domain

import (
	"context"
	"errors"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestResourceRegistrationCapabilitiesAndMissingMappings(t *testing.T) {
	for _, actions := range [][]ResourceAction{{ResourceQuery}, {ResourceCreate, ResourceDelete}, {ResourceCreate, ResourceUpdate, ResourceDelete, ResourceQuery}} {
		definition := &ResourceDefinition{Operations: map[ResourceAction]*ResourceOperation{}}
		for _, action := range actions {
			definition.Operations[action] = &ResourceOperation{Protocol: ResourceProtocol{Path: "/test", RequestFields: map[string]string{"id": "id"}, ResponseFields: map[string][]string{"id": {"id"}}, SuccessField: "code", SuccessValue: "0", DataField: "data"}, Handler: func(context.Context, *model.ProviderAccount, ResourceInput) ([]RemoteResource, error) {
				return nil, nil
			}}
		}
		if err := definition.Validate(ResourceSignatures); err != nil {
			t.Fatal(err)
		}
		cap := definition.Capability()
		if cap.Query != (definition.Operations[ResourceQuery] != nil) || cap.Update != (definition.Operations[ResourceUpdate] != nil) {
			t.Fatalf("wrong capabilities: %+v", cap)
		}
		if definition.Operations[ResourceUpdate] == nil {
			if _, err := definition.Execute(context.Background(), nil, ResourceUpdate, ResourceInput{}); !errors.Is(err, ErrResourceUnsupported) {
				t.Fatalf("unsupported operation: %v", err)
			}
		}
		for _, op := range definition.Operations {
			op.Handler = nil
			break
		}
		if definition.Validate(ResourceSignatures) == nil {
			t.Fatal("missing handler accepted")
		}
	}
	bad := &ResourceDefinition{Operations: map[ResourceAction]*ResourceOperation{ResourceDelete: {Protocol: ResourceProtocol{Path: "/delete", SuccessField: "code", SuccessValue: "0"}, Handler: func(context.Context, *model.ProviderAccount, ResourceInput) ([]RemoteResource, error) {
		return nil, nil
	}}}}
	if bad.Validate(ResourceSignatures) == nil {
		t.Fatal("delete without identity mapping accepted")
	}
}

type registrationCodec struct{ version string }

func (c registrationCodec) Version() string { return c.version }
func (c registrationCodec) Parse(content string, _ *model.ProviderAccount) (*ParsedTemplate, error) {
	return &ParsedTemplate{Content: content}, nil
}
func TestNativeCodecRegistrationIsIndependentOfCRUD(t *testing.T) {
	r := &ProviderRegistry{providers: map[string]*ProviderMeta{}}
	meta := &ProviderMeta{Code: "parse-only", Name: "本地模板", Type: "sms", SupportsSend: true, TemplateCodec: registrationCodec{"v1"}}
	if err := r.Register(meta); err != nil {
		t.Fatal(err)
	}
	if len(meta.ResourceCapabilities()) != 0 {
		t.Fatal("codec declared remote operations")
	}
	for i, codec := range []TemplateCodec{nil, registrationCodec{}} {
		if err := r.Register(&ProviderMeta{Code: string(rune('a' + i)), Name: "incomplete", Type: "sms", SupportsSend: true, TemplateCodec: codec}); err == nil {
			t.Fatal("incomplete SMS codec accepted")
		}
	}
}
