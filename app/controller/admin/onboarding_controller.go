package admin

import (
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/service"
)

// OnboardingController serves the admin setup workflow summary.
type OnboardingController struct{}

func (c OnboardingController) GetSummary(ctx httpInterfaces.RouterContextInterface) {
	response, err := service.NewAdminOnboardingService().GetSummary()
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get onboarding summary: "+err.Error())
		return
	}
	controller.SuccessResponse(ctx, response)
}
