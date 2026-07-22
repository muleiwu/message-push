package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
)

const adminTestAppID = "admin_test"

// AdminOnboardingService provides a factual, read-only setup summary.
type AdminOnboardingService struct {
	db                 *gorm.DB
	readinessEvaluator *readiness.ChannelEvaluator
}

func NewAdminOnboardingService() *AdminOnboardingService {
	db := helper.GetDatabase()
	return &AdminOnboardingService{
		db:                 db,
		readinessEvaluator: readiness.NewChannelEvaluator(db),
	}
}

func (s *AdminOnboardingService) GetSummary() (*dto.OnboardingSummaryResponse, error) {
	messageTemplates, err := s.countStep(&model.MessageTemplate{})
	if err != nil {
		return nil, err
	}
	providerAccounts, err := s.countStep(&model.ProviderAccount{})
	if err != nil {
		return nil, err
	}
	providerTemplates, err := s.countStep(&model.ProviderTemplate{})
	if err != nil {
		return nil, err
	}
	applications, err := s.countStep(&model.Application{})
	if err != nil {
		return nil, err
	}

	var channels []*model.Channel
	if err := s.db.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}
	readinessByChannel, err := s.readinessEvaluator.EvaluateChannels(channels)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate channel readiness: %w", err)
	}

	channelStep := dto.OnboardingChannelStepCounts{}
	channelStep.Total = len(channels)
	for _, channel := range channels {
		if channel.Status == 1 {
			channelStep.Enabled++
		}
		result := readinessByChannel[channel.ID]
		if result == nil {
			channelStep.Blocked++
			continue
		}
		switch result.State {
		case constants.ChannelReadinessReady:
			channelStep.Ready++
		case constants.ChannelReadinessDegraded:
			channelStep.Degraded++
		default:
			channelStep.Blocked++
		}
	}
	channelStep.Abnormal = channelStep.Degraded + channelStep.Blocked
	channelStep.Status = channelStepStatus(channelStep)

	signatureFacts, err := s.signatureFacts()
	if err != nil {
		return nil, err
	}

	channelTypes, channelBlockers := buildChannelTypeHealth(channels, readinessByChannel)
	priority := newBlockerAccumulator()
	if signatureFacts.RequiredAccountCount > signatureFacts.ConfiguredAccountCount {
		priority.addCode(constants.OnboardingBlockerProviderSignatureNotConfigured, 1)
	}

	if providerAccounts.Enabled == 0 {
		priority.addCode(constants.OnboardingBlockerProviderAccountNotConfigured, 1)
	}
	if messageTemplates.Enabled == 0 {
		priority.addCode(constants.OnboardingBlockerMessageTemplateNotConfigured, 1)
	}
	usableProviderTemplates, err := s.countUsableProviderTemplates()
	if err != nil {
		return nil, err
	}
	providerTemplates.Abnormal = providerTemplates.Total - usableProviderTemplates
	switch {
	case usableProviderTemplates == 0:
		providerTemplates.Status = constants.OnboardingStepIncomplete
	case providerTemplates.Abnormal > 0:
		providerTemplates.Status = constants.OnboardingStepAttention
	default:
		providerTemplates.Status = constants.OnboardingStepComplete
	}
	if usableProviderTemplates == 0 {
		priority.addCode(constants.OnboardingBlockerProviderTemplateNotConfigured, 1)
	}
	readyForTest := channelStep.Ready+channelStep.Degraded > 0
	if !readyForTest {
		priority.merge(channelBlockers)
		priority.addCode(constants.OnboardingBlockerChannelNotReady, 1)
	}
	if applications.Enabled == 0 {
		priority.addCode(constants.OnboardingBlockerApplicationNotConfigured, 1)
	}
	latestTest, err := s.latestAdminTest()
	if err != nil {
		return nil, err
	}
	verification, err := s.currentChannelVerification(channels, readinessByChannel)
	if err != nil {
		return nil, err
	}
	if !verification.Verified {
		if verification.SuccessChanged {
			priority.addCode(constants.OnboardingBlockerConfigurationChanged, 1)
		} else {
			priority.addCode(constants.OnboardingBlockerAdminTestNotCompleted, 1)
		}
	}

	readyForAPISend := applications.Enabled > 0 && verification.Verified

	return &dto.OnboardingSummaryResponse{
		Steps: dto.OnboardingStepsResponse{
			MessageTemplates:   messageTemplates,
			ProviderAccounts:   providerAccounts,
			ProviderTemplates:  providerTemplates,
			ProviderSignatures: signatureFacts,
			Channels:           channelStep,
			Applications:       applications,
		},
		ReadyForTest:     readyForTest,
		ReadyForAPISend:  readyForAPISend,
		ChannelTypes:     channelTypes,
		PriorityBlockers: priority.priorityList(),
		LatestAdminTest:  latestTest,
	}, nil
}

func (s *AdminOnboardingService) countStep(value any) (dto.OnboardingStepCounts, error) {
	var total int64
	if err := s.db.Model(value).Count(&total).Error; err != nil {
		return dto.OnboardingStepCounts{}, err
	}
	var enabled int64
	if err := s.db.Model(value).Where("status = 1").Count(&enabled).Error; err != nil {
		return dto.OnboardingStepCounts{}, err
	}
	step := dto.OnboardingStepCounts{
		Total:    int(total),
		Enabled:  int(enabled),
		Abnormal: int(total - enabled),
	}
	step.Status = genericStepStatus(step)
	return step, nil
}

func (s *AdminOnboardingService) signatureFacts() (dto.OnboardingProviderSignatureStepCounts, error) {
	step := dto.OnboardingProviderSignatureStepCounts{}

	var accounts []*model.ProviderAccount
	if err := s.db.Where("status = 1").Order("id ASC").Find(&accounts).Error; err != nil {
		return step, err
	}
	requiredAccounts := make(map[uint]struct{})
	for _, account := range accounts {
		meta, err := registry.GetByCode(account.ProviderCode)
		if err != nil {
			continue
		}
		if meta.RequiresSignature {
			requiredAccounts[account.ID] = struct{}{}
		} else {
			step.NotApplicableAccountCount++
		}
	}
	step.RequiredAccountCount = len(requiredAccounts)

	configuredAccounts := make(map[uint]struct{})
	if len(requiredAccounts) > 0 {
		ids := uintSetValues(requiredAccounts)
		var signatures []*model.ProviderSignature
		if err := s.db.Where("provider_account_id IN ?", ids).Find(&signatures).Error; err != nil {
			return step, err
		}
		for _, signature := range signatures {
			step.Total++
			if signature.Status == 1 {
				step.Enabled++
			}
			if signature.Status == 1 && strings.TrimSpace(signature.SignatureCode) != "" {
				configuredAccounts[signature.ProviderAccountID] = struct{}{}
			}
		}
		step.Abnormal = step.Total - step.Enabled

		var mappings []*model.ChannelSignatureMapping
		if err := s.db.Preload("ProviderSignature").Where("status = 1 AND provider_id IN ?", ids).Find(&mappings).Error; err != nil {
			return step, err
		}
		aliasKeys := make(map[string]struct{})
		for _, mapping := range mappings {
			alias := strings.TrimSpace(mapping.SignatureName)
			signature := mapping.ProviderSignature
			if alias == "" || signature == nil || signature.Status != 1 ||
				signature.ProviderAccountID != mapping.ProviderID || strings.TrimSpace(signature.SignatureCode) == "" {
				continue
			}
			key := fmt.Sprintf("%d:%d:%s", mapping.ChannelID, mapping.ProviderID, alias)
			aliasKeys[key] = struct{}{}
		}
		step.ConfiguredAliasCount = len(aliasKeys)
	}
	step.ConfiguredAccountCount = len(configuredAccounts)

	switch {
	case step.RequiredAccountCount == 0:
		step.Status = constants.OnboardingStepNotApplicable
	case step.ConfiguredAccountCount == 0:
		step.Status = constants.OnboardingStepIncomplete
	case step.ConfiguredAccountCount < step.RequiredAccountCount || step.Abnormal > 0:
		step.Status = constants.OnboardingStepAttention
	default:
		step.Status = constants.OnboardingStepComplete
	}
	return step, nil
}

func (s *AdminOnboardingService) countUsableProviderTemplates() (int, error) {
	var templates []*model.ProviderTemplate
	if err := s.db.Preload("ProviderAccount").Where("status = 1").Find(&templates).Error; err != nil {
		return 0, err
	}
	usable := 0
	for _, providerTemplate := range templates {
		account := providerTemplate.ProviderAccount
		if account == nil || account.Status != 1 || account.ID != providerTemplate.ProviderID || strings.TrimSpace(providerTemplate.TemplateCode) == "" {
			continue
		}
		meta, err := registry.GetByCode(account.ProviderCode)
		if err != nil || meta.Type != account.ProviderType || !readiness.ProviderTemplateVariablesValid(providerTemplate) {
			continue
		}
		usable++
	}
	return usable, nil
}

func (s *AdminOnboardingService) latestAdminTest() (*dto.OnboardingLatestAdminTest, error) {
	return s.latestAdminTestForChannel(0)
}

// latestAdminTestForChannel returns the latest admin test for one channel. A
// zero channel ID preserves the onboarding-wide latest-test view.
func (s *AdminOnboardingService) latestAdminTestForChannel(channelID uint) (*dto.OnboardingLatestAdminTest, error) {
	var task model.PushTask
	query := s.db.Where("app_id = ?", adminTestAppID)
	if channelID > 0 {
		query = query.Where("channel_id = ?", channelID)
	}
	err := query.Order("created_at DESC, id DESC").First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	changed, err := s.channelConfigChangedSince(task.ChannelID, task.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &dto.OnboardingLatestAdminTest{
		TaskID:        task.TaskID,
		ChannelID:     task.ChannelID,
		Status:        task.Status,
		CreatedAt:     task.CreatedAt.Format(time.RFC3339),
		ConfigChanged: changed,
	}, nil
}

type channelVerification struct {
	Verified       bool
	SuccessChanged bool
}

// currentChannelVerification checks each currently sendable channel against
// its own latest admin test. A later test on another channel therefore cannot
// invalidate a channel that has already passed with unchanged configuration.
func (s *AdminOnboardingService) currentChannelVerification(channels []*model.Channel, results map[uint]*dto.ChannelReadinessResponse) (channelVerification, error) {
	verification := channelVerification{}
	channelIDs := make([]uint, 0, len(channels))
	for _, channel := range channels {
		result := results[channel.ID]
		if result != nil && result.State != constants.ChannelReadinessBlocked && result.ValidBindingCount > 0 {
			channelIDs = append(channelIDs, channel.ID)
		}
	}
	if len(channelIDs) == 0 {
		return verification, nil
	}

	var tasks []*model.PushTask
	if err := s.db.
		Where("app_id = ? AND channel_id IN ?", adminTestAppID, channelIDs).
		Order("created_at DESC, id DESC").
		Find(&tasks).Error; err != nil {
		return verification, err
	}
	seenChannels := make(map[uint]struct{}, len(channelIDs))
	for _, task := range tasks {
		if _, seen := seenChannels[task.ChannelID]; seen {
			continue
		}
		seenChannels[task.ChannelID] = struct{}{}
		if task.Status != constants.TaskStatusSuccess {
			continue
		}
		changed, err := s.channelConfigChangedSince(task.ChannelID, task.CreatedAt)
		if err != nil {
			return verification, err
		}
		if changed {
			verification.SuccessChanged = true
			continue
		}
		verification.Verified = true
	}
	return verification, nil
}

func (s *AdminOnboardingService) channelConfigChangedSince(channelID uint, since time.Time) (bool, error) {
	changed, err := changedRows(s.db.Unscoped().Model(&model.Channel{}).Where("id = ?", channelID), since)
	if err != nil || changed {
		return changed, err
	}

	var channel model.Channel
	if err := s.db.Unscoped().First(&channel, channelID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if channel.MessageTemplateID > 0 {
		changed, err = changedRows(s.db.Unscoped().Model(&model.MessageTemplate{}).Where("id = ?", channel.MessageTemplateID), since)
		if err != nil || changed {
			return changed, err
		}
	}

	var bindings []*model.ChannelTemplateBinding
	if err := s.db.Unscoped().Where("channel_id = ?", channelID).Find(&bindings).Error; err != nil {
		return false, err
	}
	providerTemplateIDs := make(map[uint]struct{})
	providerAccountIDs := make(map[uint]struct{})
	for _, binding := range bindings {
		providerTemplateIDs[binding.ProviderTemplateID] = struct{}{}
		providerAccountIDs[binding.ProviderID] = struct{}{}
	}
	changed, err = changedRows(s.db.Unscoped().Model(&model.ChannelTemplateBinding{}).Where("channel_id = ?", channelID), since)
	if err != nil || changed {
		return changed, err
	}
	if len(providerTemplateIDs) > 0 {
		changed, err = changedRows(s.db.Unscoped().Model(&model.ProviderTemplate{}).Where("id IN ?", uintSetValues(providerTemplateIDs)), since)
		if err != nil || changed {
			return changed, err
		}
	}
	if len(providerAccountIDs) > 0 {
		changed, err = changedRows(s.db.Unscoped().Model(&model.ProviderAccount{}).Where("id IN ?", uintSetValues(providerAccountIDs)), since)
		if err != nil || changed {
			return changed, err
		}
	}

	var mappings []*model.ChannelSignatureMapping
	if err := s.db.Unscoped().Where("channel_id = ?", channelID).Find(&mappings).Error; err != nil {
		return false, err
	}
	signatureIDs := make(map[uint]struct{})
	for _, mapping := range mappings {
		signatureIDs[mapping.ProviderSignatureID] = struct{}{}
		providerAccountIDs[mapping.ProviderID] = struct{}{}
	}
	changed, err = changedRows(s.db.Unscoped().Model(&model.ChannelSignatureMapping{}).Where("channel_id = ?", channelID), since)
	if err != nil || changed {
		return changed, err
	}
	if len(signatureIDs) > 0 {
		changed, err = changedRows(s.db.Unscoped().Model(&model.ProviderSignature{}).Where("id IN ?", uintSetValues(signatureIDs)), since)
		if err != nil || changed {
			return changed, err
		}
	}
	return false, nil
}

func changedRows(query *gorm.DB, since time.Time) (bool, error) {
	var count int64
	err := query.Where("updated_at >= ? OR deleted_at >= ?", since, since).Count(&count).Error
	return count > 0, err
}

func genericStepStatus(step dto.OnboardingStepCounts) string {
	switch {
	case step.Enabled == 0:
		return constants.OnboardingStepIncomplete
	case step.Abnormal > 0:
		return constants.OnboardingStepAttention
	default:
		return constants.OnboardingStepComplete
	}
}

func channelStepStatus(step dto.OnboardingChannelStepCounts) string {
	switch {
	case step.Ready+step.Degraded == 0:
		return constants.OnboardingStepIncomplete
	case step.Degraded+step.Blocked > 0:
		return constants.OnboardingStepAttention
	default:
		return constants.OnboardingStepComplete
	}
}

func buildChannelTypeHealth(channels []*model.Channel, results map[uint]*dto.ChannelReadinessResponse) ([]*dto.OnboardingChannelTypeHealth, *blockerAccumulator) {
	typeOrder := []string{
		constants.MessageTypeSMS,
		constants.MessageTypeEmail,
		constants.MessageTypeWeChatWork,
		constants.MessageTypeDingTalk,
		constants.MessageTypeWebhook,
		constants.MessageTypePush,
	}
	healthByType := make(map[string]*dto.OnboardingChannelTypeHealth, len(typeOrder))
	blockersByType := make(map[string]*blockerAccumulator, len(typeOrder))
	for _, messageType := range typeOrder {
		healthByType[messageType] = &dto.OnboardingChannelTypeHealth{Type: messageType, Blockers: make([]*dto.OnboardingBlockerSummary, 0)}
		blockersByType[messageType] = newBlockerAccumulator()
	}
	allBlockers := newBlockerAccumulator()

	for _, channel := range channels {
		health := healthByType[channel.Type]
		if health == nil {
			health = &dto.OnboardingChannelTypeHealth{Type: channel.Type, Blockers: make([]*dto.OnboardingBlockerSummary, 0)}
			healthByType[channel.Type] = health
			blockersByType[channel.Type] = newBlockerAccumulator()
			typeOrder = append(typeOrder, channel.Type)
		}
		health.Total++
		result := results[channel.ID]
		if result == nil {
			health.Blocked++
			continue
		}
		switch result.State {
		case constants.ChannelReadinessReady:
			health.Healthy++
		case constants.ChannelReadinessDegraded:
			health.Degraded++
		default:
			health.Blocked++
		}
		for _, blocker := range result.Blockers {
			blockersByType[channel.Type].add(blocker)
			allBlockers.add(blocker)
		}
	}

	known := make(map[string]struct{})
	orderedTypes := make([]string, 0, len(typeOrder))
	for _, messageType := range typeOrder {
		if _, exists := known[messageType]; exists {
			continue
		}
		known[messageType] = struct{}{}
		orderedTypes = append(orderedTypes, messageType)
	}
	if len(orderedTypes) > len(typeOrder) {
		sort.Strings(orderedTypes[len(typeOrder):])
	}
	items := make([]*dto.OnboardingChannelTypeHealth, 0, len(orderedTypes))
	for _, messageType := range orderedTypes {
		health := healthByType[messageType]
		health.Blockers = blockersByType[messageType].list()
		items = append(items, health)
	}
	return items, allBlockers
}

type blockerAccumulator struct {
	items map[string]*blockerAccumulatorItem
	seen  map[string]struct{}
}

type blockerAccumulatorItem struct {
	count              int
	channelIDs         map[uint]struct{}
	bindingIDs         map[uint]struct{}
	providerAccountIDs map[uint]struct{}
}

func newBlockerAccumulator() *blockerAccumulator {
	return &blockerAccumulator{
		items: make(map[string]*blockerAccumulatorItem),
		seen:  make(map[string]struct{}),
	}
}

func (a *blockerAccumulator) add(blocker *dto.ChannelReadinessBlocker) {
	if blocker == nil || blocker.Code == "" {
		return
	}
	key := fmt.Sprintf("%s:%d:%d:%d", blocker.Code, blocker.ChannelID, blocker.BindingID, blocker.ProviderAccountID)
	if _, exists := a.seen[key]; exists {
		return
	}
	a.seen[key] = struct{}{}
	item := a.ensure(blocker.Code)
	item.count++
	if blocker.ChannelID > 0 {
		item.channelIDs[blocker.ChannelID] = struct{}{}
	}
	if blocker.BindingID > 0 {
		item.bindingIDs[blocker.BindingID] = struct{}{}
	}
	if blocker.ProviderAccountID > 0 {
		item.providerAccountIDs[blocker.ProviderAccountID] = struct{}{}
	}
}

func (a *blockerAccumulator) addCode(code string, count int) {
	if code == "" || count <= 0 {
		return
	}
	a.ensure(code).count += count
}

func (a *blockerAccumulator) merge(other *blockerAccumulator) {
	if other == nil {
		return
	}
	for code, source := range other.items {
		target := a.ensure(code)
		target.count += source.count
		for id := range source.channelIDs {
			target.channelIDs[id] = struct{}{}
		}
		for id := range source.bindingIDs {
			target.bindingIDs[id] = struct{}{}
		}
		for id := range source.providerAccountIDs {
			target.providerAccountIDs[id] = struct{}{}
		}
	}
}

func (a *blockerAccumulator) ensure(code string) *blockerAccumulatorItem {
	item := a.items[code]
	if item == nil {
		item = &blockerAccumulatorItem{
			channelIDs:         make(map[uint]struct{}),
			bindingIDs:         make(map[uint]struct{}),
			providerAccountIDs: make(map[uint]struct{}),
		}
		a.items[code] = item
	}
	return item
}

func (a *blockerAccumulator) list() []*dto.OnboardingBlockerSummary {
	codes := make([]string, 0, len(a.items))
	for code := range a.items {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	result := make([]*dto.OnboardingBlockerSummary, 0, len(codes))
	for _, code := range codes {
		item := a.items[code]
		result = append(result, &dto.OnboardingBlockerSummary{
			Code:               code,
			Count:              item.count,
			ChannelIDs:         uintSetValues(item.channelIDs),
			BindingIDs:         uintSetValues(item.bindingIDs),
			ProviderAccountIDs: uintSetValues(item.providerAccountIDs),
		})
	}
	return result
}

func (a *blockerAccumulator) priorityList() []*dto.OnboardingBlockerSummary {
	result := a.list()
	sort.SliceStable(result, func(i, j int) bool {
		leftRank := onboardingBlockerRank(result[i].Code)
		rightRank := onboardingBlockerRank(result[j].Code)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return result[i].Code < result[j].Code
	})
	return result
}

func onboardingBlockerRank(code string) int {
	switch code {
	case constants.OnboardingBlockerMessageTemplateNotConfigured:
		return 10
	case constants.OnboardingBlockerProviderAccountNotConfigured:
		return 20
	case constants.OnboardingBlockerProviderTemplateNotConfigured:
		return 30
	case constants.OnboardingBlockerProviderSignatureNotConfigured:
		return 40
	case constants.OnboardingBlockerChannelNotReady:
		return 50
	case constants.OnboardingBlockerAdminTestNotCompleted:
		return 80
	case constants.OnboardingBlockerConfigurationChanged:
		return 90
	case constants.OnboardingBlockerApplicationNotConfigured:
		return 100
	default:
		// Concrete channel blockers sit after the aggregate channel gate and
		// before test/application blockers.
		return 60
	}
}

func uintSetValues(values map[uint]struct{}) []uint {
	result := make([]uint, 0, len(values))
	for value := range values {
		if value > 0 {
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
