package dto

// ChannelReadinessBlocker identifies a concrete resource preventing or
// degrading channel delivery. Code is a stable machine-readable value.
type ChannelReadinessBlocker struct {
	Code               string `json:"code"`
	ChannelID          uint   `json:"channel_id,omitempty"`
	BindingID          uint   `json:"binding_id,omitempty"`
	ProviderTemplateID uint   `json:"provider_template_id,omitempty"`
	ProviderAccountID  uint   `json:"provider_account_id,omitempty"`
}

// ChannelReadinessResponse is shared by channel list/detail and onboarding.
type ChannelReadinessResponse struct {
	State                           string                     `json:"state"`
	ValidBindingCount               int                        `json:"valid_binding_count"`
	EnabledBindingCount             int                        `json:"enabled_binding_count"`
	RequiredSignatureAccountCount   int                        `json:"required_signature_account_count"`
	ConfiguredSignatureAccountCount int                        `json:"configured_signature_account_count"`
	ConfiguredSignatureAliasCount   int                        `json:"configured_signature_alias_count"`
	CommonSignatureAliases          []string                   `json:"common_signature_aliases"`
	BlockerCodes                    []string                   `json:"blocker_codes"`
	Blockers                        []*ChannelReadinessBlocker `json:"blockers"`
}

// OnboardingStepCounts describes the factual state of one setup resource.
type OnboardingStepCounts struct {
	Total    int    `json:"total"`
	Enabled  int    `json:"enabled"`
	Abnormal int    `json:"abnormal"`
	Status   string `json:"status"`
}

// OnboardingProviderSignatureStepCounts excludes accounts whose provider does
// not require a signature from the required-account denominator.
type OnboardingProviderSignatureStepCounts struct {
	OnboardingStepCounts
	RequiredAccountCount      int `json:"required_account_count"`
	ConfiguredAccountCount    int `json:"configured_account_count"`
	NotApplicableAccountCount int `json:"not_applicable_account_count"`
	ConfiguredAliasCount      int `json:"configured_alias_count"`
}

// OnboardingChannelStepCounts adds readiness-state totals to generic channel
// entity counts.
type OnboardingChannelStepCounts struct {
	OnboardingStepCounts
	Ready    int `json:"ready"`
	Degraded int `json:"degraded"`
	Blocked  int `json:"blocked"`
}

// OnboardingStepsResponse contains every branch of the setup workflow.
type OnboardingStepsResponse struct {
	MessageTemplates   OnboardingStepCounts                  `json:"message_templates"`
	ProviderAccounts   OnboardingStepCounts                  `json:"provider_accounts"`
	ProviderTemplates  OnboardingStepCounts                  `json:"provider_templates"`
	ProviderSignatures OnboardingProviderSignatureStepCounts `json:"provider_signatures"`
	Channels           OnboardingChannelStepCounts           `json:"channels"`
	Applications       OnboardingStepCounts                  `json:"applications"`
}

// OnboardingBlockerSummary aggregates stable blocker codes without losing the
// resources involved.
type OnboardingBlockerSummary struct {
	Code               string `json:"code"`
	Count              int    `json:"count"`
	ChannelIDs         []uint `json:"channel_ids,omitempty"`
	BindingIDs         []uint `json:"binding_ids,omitempty"`
	ProviderAccountIDs []uint `json:"provider_account_ids,omitempty"`
}

// OnboardingChannelTypeHealth aggregates channel readiness by message type.
type OnboardingChannelTypeHealth struct {
	Type     string                      `json:"type"`
	Total    int                         `json:"total"`
	Healthy  int                         `json:"healthy"`
	Degraded int                         `json:"degraded"`
	Blocked  int                         `json:"blocked"`
	Blockers []*OnboardingBlockerSummary `json:"blockers"`
}

// OnboardingLatestAdminTest reports the latest test and whether channel
// configuration changed after it was created.
type OnboardingLatestAdminTest struct {
	TaskID        string `json:"task_id"`
	ChannelID     uint   `json:"channel_id"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	ConfigChanged bool   `json:"config_changed"`
}

// OnboardingSummaryResponse drives the admin setup home without inferred
// client-side counts.
type OnboardingSummaryResponse struct {
	Steps            OnboardingStepsResponse        `json:"steps"`
	ReadyForTest     bool                           `json:"ready_for_test"`
	ReadyForAPISend  bool                           `json:"ready_for_api_send"`
	ChannelTypes     []*OnboardingChannelTypeHealth `json:"channel_types"`
	PriorityBlockers []*OnboardingBlockerSummary    `json:"priority_blockers"`
	LatestAdminTest  *OnboardingLatestAdminTest     `json:"latest_admin_test"`
}
