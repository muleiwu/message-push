package constants

import "testing"

func TestNormalizeResourceStatus(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int8
	}{
		{name: "disabled", in: 0, want: ResourceStatusDisabled},
		{name: "enabled", in: 1, want: ResourceStatusEnabled},
		{name: "legacy disabled", in: 2, want: ResourceStatusDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeResourceStatus(tt.in); got != tt.want {
				t.Fatalf("NormalizeResourceStatus(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
