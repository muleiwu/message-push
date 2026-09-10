package admin

import (
	"context"
	"errors"
	"strconv"
	"time"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

type smsEventAdminService interface {
	Collect(context.Context, uint, string, string, service.SMSQueryRequest) (*service.SMSCollectionResult, error)
	GetPolling(context.Context, uint) (*service.SMSPollingResponse, error)
	UpdatePolling(context.Context, uint, service.SMSPollingRequest) (*service.SMSPollingResponse, error)
	ListEvents(context.Context, uint, service.SMSListRequest) (*service.SMSEventList, error)
	RetryEvent(context.Context, uint, uint64) error
}

type SMSEventController struct{ newService func() smsEventAdminService }

func (c SMSEventController) service() smsEventAdminService {
	if c.newService != nil {
		return c.newService()
	}
	return service.NewSMSEventService()
}

func smsAdminContext(c httpInterfaces.RouterContextInterface) (context.Context, context.CancelFunc, uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		controller.ErrorResponse(c, 400, "供应商账号 ID 无效")
		return nil, func() {}, 0, false
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 50*time.Second)
	return ctx, cancel, uint(id), true
}

func smsAdminError(c httpInterfaces.RouterContextInterface, err error) {
	status := 500
	switch {
	case errors.Is(err, service.ErrSMSInvalid), errors.Is(err, domain.ErrResourceUnsupported):
		status = 400
	case errors.Is(err, service.ErrSMSNotFound):
		status = 404
	case errors.Is(err, service.ErrSMSBusy), errors.Is(err, service.ErrSMSPaused):
		status = 409
	}
	var remote *domain.RemoteResourceError
	if errors.As(err, &remote) {
		controller.BaseResponse{}.ErrorWithData(c, 502, remote.Error(), map[string]any{"provider_error_code": remote.Code, "request_id": remote.RequestID, "uncertain": remote.Uncertain})
		return
	}
	controller.ErrorResponse(c, status, err.Error())
}

func (c SMSEventController) Query(ctx httpInterfaces.RouterContextInterface) { c.collect(ctx, "query") }
func (c SMSEventController) Pull(ctx httpInterfaces.RouterContextInterface)  { c.collect(ctx, "pull") }

func (c SMSEventController) collect(ctx httpInterfaces.RouterContextInterface, source string) {
	callCtx, cancel, id, ok := smsAdminContext(ctx)
	if !ok {
		return
	}
	defer cancel()
	var request service.SMSQueryRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		controller.ErrorResponse(ctx, 400, "短信查询参数无效")
		return
	}
	response, err := c.service().Collect(callCtx, id, ctx.Param("event"), source, request)
	if err != nil {
		smsAdminError(ctx, err)
		return
	}
	controller.SuccessResponse(ctx, response)
}

func (c SMSEventController) List(ctx httpInterfaces.RouterContextInterface) {
	callCtx, cancel, id, ok := smsAdminContext(ctx)
	if !ok {
		return
	}
	defer cancel()
	var request service.SMSListRequest
	if err := ctx.ShouldBindQuery(&request); err != nil {
		controller.ErrorResponse(ctx, 400, "分页参数无效")
		return
	}
	response, err := c.service().ListEvents(callCtx, id, request)
	if err != nil {
		smsAdminError(ctx, err)
		return
	}
	controller.SuccessResponse(ctx, response)
}

func (c SMSEventController) GetPolling(ctx httpInterfaces.RouterContextInterface) {
	callCtx, cancel, id, ok := smsAdminContext(ctx)
	if !ok {
		return
	}
	defer cancel()
	response, err := c.service().GetPolling(callCtx, id)
	if err != nil {
		smsAdminError(ctx, err)
		return
	}
	controller.SuccessResponse(ctx, response)
}

func (c SMSEventController) UpdatePolling(ctx httpInterfaces.RouterContextInterface) {
	callCtx, cancel, id, ok := smsAdminContext(ctx)
	if !ok {
		return
	}
	defer cancel()
	var request service.SMSPollingRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		controller.ErrorResponse(ctx, 400, "轮询配置无效")
		return
	}
	response, err := c.service().UpdatePolling(callCtx, id, request)
	if err != nil {
		smsAdminError(ctx, err)
		return
	}
	controller.SuccessResponse(ctx, response)
}

func (c SMSEventController) Retry(ctx httpInterfaces.RouterContextInterface) {
	callCtx, cancel, id, ok := smsAdminContext(ctx)
	if !ok {
		return
	}
	defer cancel()
	eventID, err := strconv.ParseUint(ctx.Param("event"), 10, 64)
	if err != nil || eventID == 0 {
		controller.ErrorResponse(ctx, 400, "事件 ID 无效")
		return
	}
	if err := c.service().RetryEvent(callCtx, id, eventID); err != nil {
		smsAdminError(ctx, err)
		return
	}
	controller.SuccessResponse(ctx, map[string]any{"message": "已安排本地重试"})
}
