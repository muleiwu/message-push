package domain

import "testing"

func TestSignatureCapabilityIncludesRequiredProviders(t *testing.T) {
	for _, tt := range []struct {
		name                      string
		supported, required, want bool
	}{
		{"required", false, true, true},
		{"optional", true, false, true},
		{"unsupported", false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry := &ProviderRegistry{providers: map[string]*ProviderMeta{}}
			meta := &ProviderMeta{Code: tt.name, Name: tt.name, Type: "email", SupportsSignature: tt.supported, RequiresSignature: tt.required}
			if err := registry.Register(meta); err != nil {
				t.Fatal(err)
			}
			if meta.SupportsSignature != tt.want || meta.CanUseSignature() != tt.want || meta.RequiresSignature != tt.required {
				t.Fatalf("capability=%+v", meta)
			}
		})
	}
}
