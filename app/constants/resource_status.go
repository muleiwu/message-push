package constants

const (
	ResourceStatusDisabled       int8 = 0
	ResourceStatusEnabled        int8 = 1
	LegacyResourceStatusDisabled int8 = 2
)

// NormalizeResourceStatus keeps the transition contract for older clients:
// legacy status=2 means disabled, while every response and persisted value uses
// the canonical 1=enabled, 0=disabled representation.
func NormalizeResourceStatus(status int) int8 {
	if status == int(LegacyResourceStatusDisabled) {
		return ResourceStatusDisabled
	}
	return int8(status)
}
