package model

import "encoding/json"

const SendSnapshotVersion = 1

// ResolvedParamMapping records both the source and the value supplied to the sender.
// A nil Value means the provider parameter was absent; an empty string is a value.
type ResolvedParamMapping struct {
	ProviderVar string  `json:"provider_var"`
	Type        string  `json:"type"`
	SystemVar   string  `json:"system_var"`
	FixedValue  string  `json:"fixed_value"`
	Value       *string `json:"value"`
	Missing     bool    `json:"missing"`
}

// MessageContent contains only template data, never provider credentials.
type MessageContent struct {
	BindingID          uint                   `json:"binding_id"`
	ProviderTemplateID uint                   `json:"provider_template_id"`
	TemplateCode       string                 `json:"template_code"`
	TemplateName       string                 `json:"template_name"`
	TemplateContent    string                 `json:"template_content"`
	ContentType        string                 `json:"content_type"`
	OriginalParamsRaw  string                 `json:"original_params_raw"`
	OriginalParams     map[string]string      `json:"original_params"`
	ParamMapping       []ParamMappingItem     `json:"param_mapping"`
	MappedParams       map[string]string      `json:"mapped_params"`
	MappingDetails     []ResolvedParamMapping `json:"mapping_details"`
	Content            string                 `json:"content"`
	UnavailableReason  string                 `json:"unavailable_reason,omitempty"`
	Warnings           []string               `json:"warnings,omitempty"`
}

// SendSnapshot is immutable once attached to the log for a send attempt.
type SendSnapshot struct {
	MessageContent
	Version           int    `json:"version"`
	CapturedAt        string `json:"captured_at"`
	ProviderAccountID uint   `json:"provider_account_id"`
	ProviderName      string `json:"provider_name"`
	ProviderCode      string `json:"provider_code"`
	SignatureAlias    string `json:"signature_alias"`
	SignatureValue    string `json:"signature_value"`
}

// JSON returns SQL NULL for non-send logs (for example callback rule actions).
func (s *SendSnapshot) JSON() *string {
	if s == nil {
		return nil
	}
	// The snapshot contains only strings, integers, booleans and their collections.
	data, _ := json.Marshal(s)
	value := string(data)
	return &value
}
