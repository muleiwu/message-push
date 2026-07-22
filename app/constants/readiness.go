package constants

// Channel readiness states are part of the admin API contract.
const (
	ChannelReadinessReady    = "ready"
	ChannelReadinessDegraded = "degraded"
	ChannelReadinessBlocked  = "blocked"
)

// Stable channel readiness blocker codes. Keep these values backward compatible;
// admin clients use them for routing users to the resource that needs attention.
const (
	ReadinessBlockerChannelDisabled                  = "CHANNEL_DISABLED"
	ReadinessBlockerUnsupportedChannelType           = "UNSUPPORTED_CHANNEL_TYPE"
	ReadinessBlockerMessageTemplateMissing           = "MESSAGE_TEMPLATE_MISSING"
	ReadinessBlockerMessageTemplateDisabled          = "MESSAGE_TEMPLATE_DISABLED"
	ReadinessBlockerMessageTemplateVariablesInvalid  = "MESSAGE_TEMPLATE_VARIABLES_INVALID"
	ReadinessBlockerNoBindings                       = "NO_ACTIVE_BINDING"
	ReadinessBlockerNoValidBindings                  = "NO_ACTIVE_BINDING"
	ReadinessBlockerBindingDisabled                  = "BINDING_DISABLED"
	ReadinessBlockerBindingInactive                  = "BINDING_INACTIVE"
	ReadinessBlockerBindingWeightInvalid             = "BINDING_WEIGHT_INVALID"
	ReadinessBlockerProviderTemplateMissing          = "PROVIDER_TEMPLATE_UNAVAILABLE"
	ReadinessBlockerProviderTemplateDisabled         = "PROVIDER_TEMPLATE_UNAVAILABLE"
	ReadinessBlockerProviderTemplateCodeMissing      = "PROVIDER_TEMPLATE_UNAVAILABLE"
	ReadinessBlockerProviderTemplateVariablesInvalid = "PROVIDER_TEMPLATE_UNAVAILABLE"
	ReadinessBlockerProviderAccountMissing           = "PROVIDER_ACCOUNT_UNAVAILABLE"
	ReadinessBlockerProviderAccountDisabled          = "PROVIDER_ACCOUNT_UNAVAILABLE"
	ReadinessBlockerProviderAccountMismatch          = "PROVIDER_RELATION_MISMATCH"
	ReadinessBlockerProviderTypeMismatch             = "PROVIDER_RELATION_MISMATCH"
	ReadinessBlockerProviderNotRegistered            = "PROVIDER_ACCOUNT_UNAVAILABLE"
	ReadinessBlockerParamMappingInvalid              = "PARAM_MAPPING_INVALID"
	ReadinessBlockerParamMappingIncomplete           = "PARAM_MAPPING_INVALID"
	ReadinessBlockerSignatureMappingInvalid          = "SIGNATURE_REQUIRED"
	ReadinessBlockerSignatureAliasMissing            = "SIGNATURE_REQUIRED"
	ReadinessBlockerSignatureAliasNotCommon          = "SIGNATURE_ALIAS_NOT_SHARED"
	ReadinessBlockerSignatureRequired                = "SIGNATURE_REQUIRED"
	ReadinessBlockerSignatureAliasInvalid            = "SIGNATURE_ALIAS_NOT_SHARED"
)

// Stable onboarding step status codes.
const (
	OnboardingStepComplete      = "complete"
	OnboardingStepAttention     = "attention"
	OnboardingStepIncomplete    = "incomplete"
	OnboardingStepNotApplicable = "not_applicable"
)

// Stable onboarding-level blocker codes.
const (
	OnboardingBlockerApplicationNotConfigured       = "NO_ACTIVE_APPLICATION"
	OnboardingBlockerMessageTemplateNotConfigured   = "NO_ACTIVE_MESSAGE_TEMPLATE"
	OnboardingBlockerProviderAccountNotConfigured   = "NO_ACTIVE_PROVIDER_ACCOUNT"
	OnboardingBlockerProviderTemplateNotConfigured  = "NO_USABLE_PROVIDER_TEMPLATE"
	OnboardingBlockerProviderSignatureNotConfigured = "SIGNATURE_REQUIRED"
	OnboardingBlockerChannelNotReady                = "NO_TESTABLE_CHANNEL"
	OnboardingBlockerAdminTestNotCompleted          = "ADMIN_TEST_NOT_COMPLETED"
	OnboardingBlockerConfigurationChanged           = "CONFIGURATION_CHANGED_SINCE_TEST"
)
