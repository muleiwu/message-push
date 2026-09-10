package controller

import (
	"errors"
	"net/http"
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/modules/channel"
)

// ChannelController exposes the read-only channel catalog to authenticated applications.
type ChannelController struct{}

func (ChannelController) ListChannels(c httpInterfaces.RouterContextInterface) {
	req := dto.PublicChannelListRequest{Page: 1, PageSize: 20}
	if err := c.ShouldBindQuery(&req); err != nil {
		ErrorResponse(c, http.StatusBadRequest, "invalid channel catalog query")
		return
	}
	response, err := channel.GetCatalogService().ListChannels(c.Request().Context(), req)
	if err != nil {
		catalogError(c, err)
		return
	}
	SuccessWithData(c, response)
}

func (ChannelController) GetChannel(c httpInterfaces.RouterContextInterface) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize-1)
	if err != nil || id == 0 {
		ErrorResponse(c, http.StatusBadRequest, "invalid channel ID")
		return
	}
	response, err := channel.GetCatalogService().GetChannel(c.Request().Context(), uint(id))
	if err != nil {
		catalogError(c, err)
		return
	}
	SuccessWithData(c, response)
}

func catalogError(c httpInterfaces.RouterContextInterface, err error) {
	switch {
	case errors.Is(err, channel.ErrInvalidCatalogRequest):
		ErrorResponse(c, http.StatusBadRequest, "invalid channel catalog query")
	case errors.Is(err, channel.ErrCatalogChannelNotFound):
		ErrorResponse(c, http.StatusNotFound, "channel not found")
	default:
		c.Error(err)
		ErrorResponse(c, http.StatusInternalServerError, "failed to query channel catalog")
	}
}
