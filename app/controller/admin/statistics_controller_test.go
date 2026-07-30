package admin

import (
	"fmt"
	"net/http"
	"testing"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
)

type statisticsControllerTestService struct {
	adminStatisticsService
	err error
}

func (s statisticsControllerTestService) GetStatistics(*dto.StatisticsRequest) (*dto.StatisticsResponse, error) {
	return nil, s.err
}

type statisticsControllerTestContext struct {
	httpInterfaces.RouterContextInterface
	status int
	body   any
}

func (c *statisticsControllerTestContext) ShouldBindQuery(any) error {
	return nil
}

func (c *statisticsControllerTestContext) JSON(status int, body any) {
	c.status = status
	c.body = body
}

func TestStatisticsControllerReturnsBadRequestForInvalidStatisticsFilters(t *testing.T) {
	ctx := &statisticsControllerTestContext{}
	controller := StatisticsController{
		newService: func() adminStatisticsService {
			return statisticsControllerTestService{
				err: fmt.Errorf("%w: date range cannot exceed 90 days", service.ErrInvalidStatisticsRequest),
			}
		},
	}

	controller.GetStatistics(ctx)

	if ctx.status != http.StatusBadRequest {
		t.Fatalf("HTTP status = %d, want %d", ctx.status, http.StatusBadRequest)
	}
	response, ok := ctx.body.(dto.Response)
	if !ok {
		t.Fatalf("response body type = %T, want dto.Response", ctx.body)
	}
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response code = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
