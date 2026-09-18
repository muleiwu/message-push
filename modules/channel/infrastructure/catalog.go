package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	"cnb.cool/mliev/push/message-push/modules/channel/domain"
	"gorm.io/gorm"
)

var _ domain.CatalogService = (*CatalogService)(nil)

// CatalogService reads public channel configuration without touching delivery state.
type CatalogService struct {
	db *gorm.DB
}

func NewCatalogService(db *gorm.DB) *CatalogService {
	return &CatalogService{db: db}
}

func (s *CatalogService) ListChannels(ctx context.Context, req dto.PublicChannelListRequest) (*dto.PublicChannelListResponse, error) {
	if req.Page < 1 || req.PageSize < 1 || req.PageSize > 100 ||
		(req.Page-1) > int(^uint(0)>>1)/req.PageSize {
		return nil, fmt.Errorf("%w: invalid pagination", domain.ErrInvalidCatalogRequest)
	}
	if req.Type != "" && !constants.IsValidMessageType(req.Type) {
		return nil, fmt.Errorf("%w: unsupported channel type", domain.ErrInvalidCatalogRequest)
	}

	db := s.db.WithContext(ctx)
	query := db.Model(&model.Channel{}).Where("status = ?", constants.ResourceStatusEnabled)
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	response := &dto.PublicChannelListResponse{
		Items: make([]dto.PublicChannelResponse, 0),
		Page:  req.Page,
		Size:  req.PageSize,
	}
	if err := query.Count(&response.Total).Error; err != nil {
		return nil, fmt.Errorf("count catalog channels: %w", err)
	}
	var channels []*model.Channel
	if err := query.Preload("MessageTemplate").Order("id DESC").
		Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("load catalog channels: %w", err)
	}
	states, err := readiness.NewChannelEvaluator(db).EvaluateChannels(channels)
	if err != nil {
		return nil, fmt.Errorf("evaluate catalog channels: %w", err)
	}
	for _, channel := range channels {
		state, ok := states[channel.ID]
		if !ok {
			return nil, fmt.Errorf("catalog channel %d changed during query", channel.ID)
		}
		response.Items = append(response.Items, publicChannel(channel, state))
	}
	return response, nil
}

func (s *CatalogService) GetChannel(ctx context.Context, id uint) (*dto.PublicChannelDetailResponse, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: channel ID must be positive", domain.ErrInvalidCatalogRequest)
	}
	db := s.db.WithContext(ctx)
	var channel model.Channel
	err := db.Preload("MessageTemplate").Where("status = ?", constants.ResourceStatusEnabled).First(&channel, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCatalogChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load catalog channel: %w", err)
	}
	state, err := readiness.NewChannelEvaluator(db).EvaluateChannel(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrCatalogChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("evaluate catalog channel: %w", err)
	}
	response := &dto.PublicChannelDetailResponse{
		PublicChannelResponse: publicChannel(&channel, state),
		SignatureNames:        make([]string, 0),
	}
	if template := channel.MessageTemplate; template != nil {
		variables, err := template.GetVariables()
		if err != nil || slices.Contains(state.BlockerCodes, constants.ReadinessBlockerMessageTemplateVariablesInvalid) {
			variables = nil
		} else if variables == nil {
			variables = make([]string, 0)
		}
		contentType := template.ContentType
		if contentType == "" {
			contentType = "text"
		}
		response.Template = &dto.PublicChannelTemplate{
			ID:           template.ID,
			TemplateName: template.TemplateName,
			ContentType:  contentType,
			Content:      template.Content,
			Variables:    variables,
			Description:  template.Description,
		}
	}
	// This is the same eligible required-signature group used by ValidateForSend:
	// blocked channels and groups without common aliases have no required path.
	if state.State != constants.ChannelReadinessBlocked && state.RequiredSignatureAccountCount > 0 {
		response.SignatureNames = append(response.SignatureNames, state.CommonSignatureAliases...)
		response.SignatureRequired = len(response.SignatureNames) > 0
	}
	if state.State != constants.ChannelReadinessBlocked && state.OptionalSignatureAccountCount > 0 {
		if !response.SignatureRequired {
			response.SignatureNames = append(response.SignatureNames, state.OptionalSignatureAliases...)
		} else {
			response.SignatureNames = slices.DeleteFunc(response.SignatureNames, func(alias string) bool {
				return !slices.Contains(state.OptionalSignatureAliases, alias)
			})
		}
	}
	return response, nil
}

func publicChannel(channel *model.Channel, state *dto.ChannelReadinessResponse) dto.PublicChannelResponse {
	response := dto.PublicChannelResponse{
		ID:                channel.ID,
		Name:              channel.Name,
		Type:              channel.Type,
		MessageTemplateID: channel.MessageTemplateID,
		Readiness: dto.PublicChannelReadiness{
			State:        state.State,
			BlockerCodes: append([]string{}, state.BlockerCodes...),
		},
	}
	if channel.MessageTemplate != nil {
		response.TemplateName = channel.MessageTemplate.TemplateName
	}
	return response
}
