package domain

import (
	"context"
	"errors"

	"cnb.cool/mliev/push/message-push/app/dto"
)

var (
	ErrInvalidCatalogRequest  = errors.New("invalid channel catalog request")
	ErrCatalogChannelNotFound = errors.New("channel not found")
)

// CatalogService is the read-only channel configuration available to API applications.
type CatalogService interface {
	ListChannels(ctx context.Context, req dto.PublicChannelListRequest) (*dto.PublicChannelListResponse, error)
	GetChannel(ctx context.Context, id uint) (*dto.PublicChannelDetailResponse, error)
}
