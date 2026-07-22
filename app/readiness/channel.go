// Package readiness contains the shared, read-only channel readiness rules used
// by admin APIs and the delivery path.
package readiness

import (
	"fmt"
	"sort"
	"strings"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
)

// ValidationError exposes stable readiness codes to callers without coupling
// them to translated admin copy.
type ValidationError struct {
	Codes []string
}

func (e *ValidationError) Error() string {
	return "channel readiness validation failed: " + strings.Join(e.Codes, ",")
}

// ChannelEvaluator evaluates channels against their templates, bindings,
// provider accounts, parameter mappings, and required signature aliases.
type ChannelEvaluator struct {
	db *gorm.DB
}

func NewChannelEvaluator(db *gorm.DB) *ChannelEvaluator {
	return &ChannelEvaluator{db: db}
}

type channelEvaluation struct {
	response                *dto.ChannelReadinessResponse
	commonAliases           map[string]struct{}
	validBindingIDs         map[uint]struct{}
	hasEligibleRequiredPath bool
}

// DeliveryEligibility is alias-independent. Required-signature bindings are
// present only when every required account shares at least one alias.
type DeliveryEligibility struct {
	ValidBindingIDs        []uint
	CommonSignatureAliases []string
}

// EvaluateChannel evaluates a single persisted channel.
func (e *ChannelEvaluator) EvaluateChannel(channelID uint) (*dto.ChannelReadinessResponse, error) {
	evaluations, err := e.loadAndEvaluate([]uint{channelID})
	if err != nil {
		return nil, err
	}
	evaluation, ok := evaluations[channelID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return evaluation.response, nil
}

// EvaluateChannels evaluates already loaded channels in batches. MessageTemplate
// is loaded by this method when the caller did not preload it.
func (e *ChannelEvaluator) EvaluateChannels(channels []*model.Channel) (map[uint]*dto.ChannelReadinessResponse, error) {
	ids := make([]uint, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.ID > 0 {
			ids = append(ids, channel.ID)
		}
	}
	if len(ids) == 0 {
		return map[uint]*dto.ChannelReadinessResponse{}, nil
	}

	evaluations, err := e.loadAndEvaluate(ids)
	if err != nil {
		return nil, err
	}
	responses := make(map[uint]*dto.ChannelReadinessResponse, len(evaluations))
	for id, evaluation := range evaluations {
		responses[id] = evaluation.response
	}
	return responses, nil
}

// GetDeliveryEligibility returns the static candidate set used by the selector.
// The requested alias never changes this set.
func (e *ChannelEvaluator) GetDeliveryEligibility(channelID uint) (*DeliveryEligibility, error) {
	evaluations, err := e.loadAndEvaluate([]uint{channelID})
	if err != nil {
		return nil, err
	}
	evaluation, ok := evaluations[channelID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &DeliveryEligibility{
		ValidBindingIDs:        uintSetValues(evaluation.validBindingIDs),
		CommonSignatureAliases: sortedStringSet(evaluation.commonAliases),
	}, nil
}

// ValidateForSend applies the same rules as the admin readiness API and also
// verifies that a required signature alias is valid for every selectable
// signature-requiring provider account.
func (e *ChannelEvaluator) ValidateForSend(channelID uint, signatureAlias string) error {
	evaluations, err := e.loadAndEvaluate([]uint{channelID})
	if err != nil {
		return err
	}
	evaluation, ok := evaluations[channelID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if evaluation.response.State == constants.ChannelReadinessBlocked {
		return &ValidationError{Codes: append([]string(nil), evaluation.response.BlockerCodes...)}
	}

	if !evaluation.hasEligibleRequiredPath {
		return nil
	}

	alias := strings.TrimSpace(signatureAlias)
	if alias == "" {
		return &ValidationError{Codes: []string{constants.ReadinessBlockerSignatureRequired}}
	}
	if _, ok := evaluation.commonAliases[alias]; !ok {
		return &ValidationError{Codes: []string{constants.ReadinessBlockerSignatureAliasNotCommon}}
	}
	return nil
}

func (e *ChannelEvaluator) loadAndEvaluate(channelIDs []uint) (map[uint]*channelEvaluation, error) {
	var channels []*model.Channel
	if err := e.db.Preload("MessageTemplate").Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return map[uint]*channelEvaluation{}, nil
	}

	ids := make([]uint, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.ID)
	}

	var bindings []*model.ChannelTemplateBinding
	if err := e.db.
		Preload("ProviderTemplate").
		Preload("ProviderTemplate.ProviderAccount").
		Where("channel_id IN ?", ids).
		Order("channel_id ASC, id ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}

	var mappings []*model.ChannelSignatureMapping
	if err := e.db.
		Preload("ProviderSignature").
		Where("channel_id IN ?", ids).
		Order("channel_id ASC, id ASC").
		Find(&mappings).Error; err != nil {
		return nil, err
	}

	bindingsByChannel := make(map[uint][]*model.ChannelTemplateBinding)
	for _, binding := range bindings {
		bindingsByChannel[binding.ChannelID] = append(bindingsByChannel[binding.ChannelID], binding)
	}
	mappingsByChannel := make(map[uint][]*model.ChannelSignatureMapping)
	for _, mapping := range mappings {
		mappingsByChannel[mapping.ChannelID] = append(mappingsByChannel[mapping.ChannelID], mapping)
	}

	result := make(map[uint]*channelEvaluation, len(channels))
	for _, channel := range channels {
		result[channel.ID] = evaluateChannel(channel, bindingsByChannel[channel.ID], mappingsByChannel[channel.ID])
	}
	return result, nil
}

func evaluateChannel(channel *model.Channel, bindings []*model.ChannelTemplateBinding, mappings []*model.ChannelSignatureMapping) *channelEvaluation {
	response := &dto.ChannelReadinessResponse{
		State:        constants.ChannelReadinessReady,
		BlockerCodes: make([]string, 0),
		Blockers:     make([]*dto.ChannelReadinessBlocker, 0),
	}

	addChannelBlocker := func(code string) {
		response.Blockers = append(response.Blockers, &dto.ChannelReadinessBlocker{
			Code:      code,
			ChannelID: channel.ID,
		})
	}

	if channel.Status != 1 {
		addChannelBlocker(constants.ReadinessBlockerChannelDisabled)
	}
	if !constants.IsValidMessageType(channel.Type) {
		addChannelBlocker(constants.ReadinessBlockerUnsupportedChannelType)
	}

	systemVariables := make([]string, 0)
	if channel.MessageTemplate == nil {
		addChannelBlocker(constants.ReadinessBlockerMessageTemplateMissing)
	} else {
		if channel.MessageTemplate.Status != 1 {
			addChannelBlocker(constants.ReadinessBlockerMessageTemplateDisabled)
		}
		variables, err := channel.MessageTemplate.GetVariables()
		if err != nil || !validVariableNames(variables) {
			addChannelBlocker(constants.ReadinessBlockerMessageTemplateVariablesInvalid)
		} else {
			systemVariables = variables
		}
	}

	if len(bindings) == 0 {
		addChannelBlocker(constants.ReadinessBlockerNoBindings)
	}

	validBindings := make([]*model.ChannelTemplateBinding, 0, len(bindings))
	for _, binding := range bindings {
		// A manually disabled binding is outside the configured delivery set. It
		// must not degrade a channel merely because it is kept as a spare. An
		// enabled but auto-disabled/invalid binding is still reported below.
		if binding.Status != 1 {
			continue
		}
		response.EnabledBindingCount++

		issues := ValidateBinding(channel.Type, systemVariables, binding)
		if len(issues) == 0 {
			validBindings = append(validBindings, binding)
			continue
		}
		for _, code := range issues {
			response.Blockers = append(response.Blockers, bindingBlocker(code, channel.ID, binding))
		}
	}
	signatureResult := evaluateRequiredSignatures(response, channel.ID, validBindings, mappings)
	response.CommonSignatureAliases = sortedStringSet(signatureResult.commonAliases)
	finalBindings := make([]*model.ChannelTemplateBinding, 0, len(validBindings))
	hasEligibleRequiredPath := false
	requiredGroupEligible := response.RequiredSignatureAccountCount > 0 && len(signatureResult.commonAliases) > 0
	for _, binding := range validBindings {
		account := binding.ProviderTemplate.ProviderAccount
		meta, err := registry.GetByCode(account.ProviderCode)
		if err == nil && meta.RequiresSignature {
			if !requiredGroupEligible {
				continue
			}
			hasEligibleRequiredPath = true
		}
		finalBindings = append(finalBindings, binding)
	}
	response.ValidBindingCount = len(finalBindings)
	if len(bindings) > 0 && len(finalBindings) == 0 {
		addChannelBlocker(constants.ReadinessBlockerNoValidBindings)
	}

	validBindingIDs := make(map[uint]struct{}, len(finalBindings))
	for _, binding := range finalBindings {
		validBindingIDs[binding.ID] = struct{}{}
	}

	finalizeReadiness(response)
	if response.State == constants.ChannelReadinessBlocked {
		// Hard channel/template gates and an empty usable set are fail-closed.
		// Keep facts and blockers, but never expose a selector candidate while
		// the response says the channel cannot send.
		response.ValidBindingCount = 0
		validBindingIDs = make(map[uint]struct{})
		hasEligibleRequiredPath = false
	}

	return &channelEvaluation{
		response:                response,
		commonAliases:           signatureResult.commonAliases,
		validBindingIDs:         validBindingIDs,
		hasEligibleRequiredPath: hasEligibleRequiredPath,
	}
}

// ValidateBinding returns stable blocker codes for a binding. It is deliberately
// pure so the selector can enforce exactly the same runtime eligibility rules.
func ValidateBinding(channelType string, systemVariables []string, binding *model.ChannelTemplateBinding) []string {
	issues := make([]string, 0)
	if binding == nil {
		return []string{constants.ReadinessBlockerNoValidBindings}
	}
	if binding.Status != 1 {
		issues = append(issues, constants.ReadinessBlockerBindingDisabled)
	}
	if binding.IsActive != 1 {
		issues = append(issues, constants.ReadinessBlockerBindingInactive)
	}
	if binding.Weight <= 0 {
		issues = append(issues, constants.ReadinessBlockerBindingWeightInvalid)
	}

	providerTemplate := binding.ProviderTemplate
	if providerTemplate == nil {
		return append(issues, constants.ReadinessBlockerProviderTemplateMissing)
	}
	if providerTemplate.Status != 1 {
		issues = append(issues, constants.ReadinessBlockerProviderTemplateDisabled)
	}
	if strings.TrimSpace(providerTemplate.TemplateCode) == "" {
		issues = append(issues, constants.ReadinessBlockerProviderTemplateCodeMissing)
	}
	if providerTemplate.ProviderID == 0 || binding.ProviderID != providerTemplate.ProviderID {
		issues = append(issues, constants.ReadinessBlockerProviderAccountMismatch)
	}

	providerVariables, err := providerTemplate.GetVariables()
	providerVariablesValid := err == nil && ProviderTemplateVariablesValid(providerTemplate)
	if !providerVariablesValid {
		issues = append(issues, constants.ReadinessBlockerProviderTemplateVariablesInvalid)
	}

	account := providerTemplate.ProviderAccount
	if account == nil {
		issues = append(issues, constants.ReadinessBlockerProviderAccountMissing)
	} else {
		if account.ID != binding.ProviderID || account.ID != providerTemplate.ProviderID {
			issues = append(issues, constants.ReadinessBlockerProviderAccountMismatch)
		}
		if account.Status != 1 {
			issues = append(issues, constants.ReadinessBlockerProviderAccountDisabled)
		}
		if account.ProviderType != channelType {
			issues = append(issues, constants.ReadinessBlockerProviderTypeMismatch)
		}
		meta, metaErr := registry.GetByCode(account.ProviderCode)
		if metaErr != nil {
			issues = append(issues, constants.ReadinessBlockerProviderNotRegistered)
		} else if meta.Type != channelType {
			issues = append(issues, constants.ReadinessBlockerProviderTypeMismatch)
		}
	}

	if providerVariablesValid {
		mappingIssues := validateParamMapping(systemVariables, providerVariables, binding)
		issues = append(issues, mappingIssues...)
	}

	return uniqueSortedStrings(issues)
}

// ValidateBindingParamMapping exposes the same parameter-mapping rule for admin
// create/update validation without requiring an enabled binding.
func ValidateBindingParamMapping(systemVariables []string, binding *model.ChannelTemplateBinding) []string {
	if binding == nil || binding.ProviderTemplate == nil {
		return []string{constants.ReadinessBlockerProviderTemplateMissing}
	}
	providerVariables, err := binding.ProviderTemplate.GetVariables()
	if err != nil || !ProviderTemplateVariablesValid(binding.ProviderTemplate) {
		return []string{constants.ReadinessBlockerProviderTemplateVariablesInvalid}
	}
	return validateParamMapping(systemVariables, providerVariables, binding)
}

func validateParamMapping(systemVariables, providerVariables []string, binding *model.ChannelTemplateBinding) []string {
	mapping, err := binding.GetParamMapping()
	if err != nil {
		return []string{constants.ReadinessBlockerParamMappingInvalid}
	}

	systemSet := stringSet(systemVariables)
	providerSet := stringSet(providerVariables)
	if len(providerVariables) == 0 {
		if len(mapping) > 0 {
			return []string{constants.ReadinessBlockerParamMappingInvalid}
		}
		return nil
	}

	if len(mapping) == 0 {
		for _, providerVariable := range providerVariables {
			if _, ok := systemSet[providerVariable]; !ok {
				return []string{constants.ReadinessBlockerParamMappingIncomplete}
			}
		}
		return nil
	}

	invalid := false
	seen := make(map[string]struct{}, len(mapping))
	for _, item := range mapping {
		providerVariable := strings.TrimSpace(item.ProviderVar)
		if providerVariable == "" {
			invalid = true
			continue
		}
		if _, ok := providerSet[providerVariable]; !ok {
			invalid = true
		}
		if _, duplicate := seen[providerVariable]; duplicate {
			invalid = true
		}
		seen[providerVariable] = struct{}{}

		switch item.Type {
		case model.ParamMappingTypeFixed:
			// Empty fixed values are allowed; some providers use them deliberately.
		case model.ParamMappingTypeMapping:
			systemVariable := strings.TrimSpace(item.SystemVar)
			if systemVariable == "" {
				invalid = true
				continue
			}
			if _, ok := systemSet[systemVariable]; !ok {
				invalid = true
			}
		default:
			invalid = true
		}
	}

	incomplete := false
	for providerVariable := range providerSet {
		if _, ok := seen[providerVariable]; !ok {
			incomplete = true
		}
	}

	issues := make([]string, 0, 2)
	if invalid {
		issues = append(issues, constants.ReadinessBlockerParamMappingInvalid)
	}
	if incomplete {
		issues = append(issues, constants.ReadinessBlockerParamMappingIncomplete)
	}
	return issues
}

type signatureEvaluation struct {
	aliasesByAccount map[uint]map[string]struct{}
	commonAliases    map[string]struct{}
}

func evaluateRequiredSignatures(response *dto.ChannelReadinessResponse, channelID uint, bindings []*model.ChannelTemplateBinding, mappings []*model.ChannelSignatureMapping) signatureEvaluation {
	requiredAccounts := make(map[uint]struct{})
	for _, binding := range bindings {
		account := binding.ProviderTemplate.ProviderAccount
		meta, err := registry.GetByCode(account.ProviderCode)
		if err == nil && meta.RequiresSignature {
			requiredAccounts[account.ID] = struct{}{}
		}
	}
	response.RequiredSignatureAccountCount = len(requiredAccounts)
	if len(requiredAccounts) == 0 {
		return signatureEvaluation{
			aliasesByAccount: map[uint]map[string]struct{}{},
			commonAliases:    map[string]struct{}{},
		}
	}

	aliasesByAccount := make(map[uint]map[string]struct{}, len(requiredAccounts))
	for accountID := range requiredAccounts {
		aliasesByAccount[accountID] = make(map[string]struct{})
	}
	for _, mapping := range mappings {
		if mapping.Status != 1 {
			continue
		}
		if _, required := requiredAccounts[mapping.ProviderID]; !required {
			continue
		}

		signature := mapping.ProviderSignature
		alias := strings.TrimSpace(mapping.SignatureName)
		if signature == nil || signature.Status != 1 ||
			signature.ProviderAccountID != mapping.ProviderID ||
			alias == "" || strings.TrimSpace(signature.SignatureCode) == "" {
			// A stale redundant mapping does not remove an otherwise valid route.
			// If the account has no usable mapping, the account-level check below
			// emits SIGNATURE_REQUIRED and excludes that account's bindings.
			continue
		}
		aliasesByAccount[mapping.ProviderID][alias] = struct{}{}
	}

	var commonAliases map[string]struct{}
	accountIDs := make([]uint, 0, len(requiredAccounts))
	for accountID := range requiredAccounts {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })

	for _, accountID := range accountIDs {
		aliases := aliasesByAccount[accountID]
		if len(aliases) == 0 {
			response.Blockers = append(response.Blockers, &dto.ChannelReadinessBlocker{
				Code:              constants.ReadinessBlockerSignatureAliasMissing,
				ChannelID:         channelID,
				ProviderAccountID: accountID,
			})
			continue
		}
		response.ConfiguredSignatureAccountCount++
		if commonAliases == nil {
			commonAliases = cloneStringSet(aliases)
			continue
		}
		for alias := range commonAliases {
			if _, ok := aliases[alias]; !ok {
				delete(commonAliases, alias)
			}
		}
	}

	if commonAliases == nil {
		commonAliases = make(map[string]struct{})
	}
	if response.ConfiguredSignatureAccountCount < response.RequiredSignatureAccountCount {
		commonAliases = make(map[string]struct{})
	}
	response.ConfiguredSignatureAliasCount = len(commonAliases)
	if response.ConfiguredSignatureAccountCount == response.RequiredSignatureAccountCount && len(commonAliases) == 0 {
		response.Blockers = append(response.Blockers, &dto.ChannelReadinessBlocker{
			Code:      constants.ReadinessBlockerSignatureAliasNotCommon,
			ChannelID: channelID,
		})
	}
	return signatureEvaluation{
		aliasesByAccount: aliasesByAccount,
		commonAliases:    commonAliases,
	}
}

func finalizeReadiness(response *dto.ChannelReadinessResponse) {
	sort.SliceStable(response.Blockers, func(i, j int) bool {
		left, right := response.Blockers[i], response.Blockers[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.BindingID != right.BindingID {
			return left.BindingID < right.BindingID
		}
		return left.ProviderAccountID < right.ProviderAccountID
	})

	codes := make([]string, 0, len(response.Blockers))
	for _, blocker := range response.Blockers {
		codes = append(codes, blocker.Code)
	}
	response.BlockerCodes = uniqueSortedStrings(codes)

	hardBlockers := map[string]struct{}{
		constants.ReadinessBlockerChannelDisabled:                 {},
		constants.ReadinessBlockerUnsupportedChannelType:          {},
		constants.ReadinessBlockerMessageTemplateMissing:          {},
		constants.ReadinessBlockerMessageTemplateDisabled:         {},
		constants.ReadinessBlockerMessageTemplateVariablesInvalid: {},
		constants.ReadinessBlockerNoBindings:                      {},
	}
	for _, code := range response.BlockerCodes {
		if _, blocked := hardBlockers[code]; blocked {
			response.State = constants.ChannelReadinessBlocked
			return
		}
	}
	if len(response.BlockerCodes) > 0 {
		response.State = constants.ChannelReadinessDegraded
		return
	}
	response.State = constants.ChannelReadinessReady
}

func bindingBlocker(code string, channelID uint, binding *model.ChannelTemplateBinding) *dto.ChannelReadinessBlocker {
	blocker := &dto.ChannelReadinessBlocker{
		Code:      code,
		ChannelID: channelID,
	}
	if binding != nil {
		blocker.BindingID = binding.ID
		blocker.ProviderTemplateID = binding.ProviderTemplateID
		blocker.ProviderAccountID = binding.ProviderID
	}
	return blocker
}

func validVariableNames(variables []string) bool {
	seen := make(map[string]struct{}, len(variables))
	for _, variable := range variables {
		trimmed := strings.TrimSpace(variable)
		if trimmed == "" || trimmed != variable {
			return false
		}
		if _, duplicate := seen[variable]; duplicate {
			return false
		}
		seen[variable] = struct{}{}
	}
	return true
}

// ProviderTemplateVariablesValid is the shared structural validation used by
// onboarding, readiness, and runtime selection. Empty variable lists are valid.
func ProviderTemplateVariablesValid(providerTemplate *model.ProviderTemplate) bool {
	if providerTemplate == nil {
		return false
	}
	variables, err := providerTemplate.GetVariables()
	return err == nil && validVariableNames(variables)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func cloneStringSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for value := range source {
		result[value] = struct{}{}
	}
	return result
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uintSetValues(values map[uint]struct{}) []uint {
	result := make([]uint, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (e *ChannelEvaluator) String() string {
	return fmt.Sprintf("ChannelEvaluator(%p)", e.db)
}
