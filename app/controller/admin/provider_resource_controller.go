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

type ProviderResourceController struct{}

func resourceControllerContext(c httpInterfaces.RouterContextInterface) (context.Context, context.CancelFunc, uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		controller.ErrorResponse(c, 400, "invalid provider account id")
		return nil, func() {}, 0, false
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 45*time.Second)
	return ctx, cancel, uint(id), true
}

func resourceControllerError(c httpInterfaces.RouterContextInterface, err error) {
	status := 400
	if errors.Is(err, service.ErrResourceConflict) {
		status = 409
	}
	var remote *domain.RemoteResourceError
	if errors.As(err, &remote) {
		status = 502
		message := remote.Error()
		if remote.Uncertain {
			message += "；执行结果不确定，请先查询，勿重复提交"
		}
		controller.BaseResponse{}.ErrorWithData(c, status, message, map[string]any{"provider_error_code": remote.Code, "request_id": remote.RequestID, "uncertain": remote.Uncertain})
		return
	}
	controller.ErrorResponse(c, status, err.Error())
}

func (ProviderResourceController) Query(c httpInterfaces.RouterContextInterface) {
	ctx, cancel, id, ok := resourceControllerContext(c)
	if !ok {
		return
	}
	defer cancel()
	items, err := service.NewAdminProviderResourceService().Preview(ctx, id, domain.ResourceKind(c.Param("kind")), c.Param("remoteId"))
	if err != nil {
		resourceControllerError(c, err)
		return
	}
	controller.SuccessResponse(c, map[string]any{"items": items})
}

func (ProviderResourceController) Preview(c httpInterfaces.RouterContextInterface) {
	ctx, cancel, id, ok := resourceControllerContext(c)
	if !ok {
		return
	}
	defer cancel()
	var req struct {
		Kind domain.ResourceKind `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resourceControllerError(c, err)
		return
	}
	items, err := service.NewAdminProviderResourceService().Workspace(ctx, id, req.Kind)
	if err != nil {
		resourceControllerError(c, err)
		return
	}
	controller.SuccessResponse(c, map[string]any{"items": items})
}

func (ProviderResourceController) Parse(c httpInterfaces.RouterContextInterface) {
	ctx, cancel, id, ok := resourceControllerContext(c)
	if !ok {
		return
	}
	defer cancel()
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resourceControllerError(c, err)
		return
	}
	result, err := service.NewAdminProviderResourceService().Parse(ctx, id, req.Content)
	if err != nil {
		resourceControllerError(c, err)
		return
	}
	controller.SuccessResponse(c, result)
}

func (ProviderResourceController) Import(c httpInterfaces.RouterContextInterface) {
	ctx, cancel, id, ok := resourceControllerContext(c)
	if !ok {
		return
	}
	defer cancel()
	var req service.ResourceImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resourceControllerError(c, err)
		return
	}
	result, err := service.NewAdminProviderResourceService().Import(ctx, id, req)
	if err != nil {
		resourceControllerError(c, err)
		return
	}
	controller.SuccessResponse(c, result)
}

func (ProviderResourceController) Create(c httpInterfaces.RouterContextInterface) {
	mutateProviderResource(c, domain.ResourceCreate)
}
func (ProviderResourceController) Update(c httpInterfaces.RouterContextInterface) {
	mutateProviderResource(c, domain.ResourceUpdate)
}
func (ProviderResourceController) Delete(c httpInterfaces.RouterContextInterface) {
	mutateProviderResource(c, domain.ResourceDelete)
}

func mutateProviderResource(c httpInterfaces.RouterContextInterface, action domain.ResourceAction) {
	ctx, cancel, id, ok := resourceControllerContext(c)
	if !ok {
		return
	}
	defer cancel()
	var req service.ResourceMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resourceControllerError(c, err)
		return
	}
	req.ID = c.Param("remoteId")
	result, err := service.NewAdminProviderResourceService().Mutate(ctx, id, domain.ResourceKind(c.Param("kind")), action, req)
	if err != nil {
		resourceControllerError(c, err)
		return
	}
	controller.SuccessResponse(c, result)
}
