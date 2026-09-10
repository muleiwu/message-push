package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerr "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type tencentClientFactory func(*model.ProviderAccount) (*sms.Client, error)

func newTencentClient(account *model.ProviderAccount) (*sms.Client, error) {
	if account == nil {
		return nil, fmt.Errorf("供应商账号不存在")
	}
	config, err := account.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("腾讯云账号配置无效")
	}
	secretID, _ := config["secret_id"].(string)
	secretKey, _ := config["secret_key"].(string)
	region, _ := config["region"].(string)
	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("请先配置腾讯云 SecretId 和 SecretKey")
	}
	if region == "" {
		region = "ap-guangzhou"
	}
	p := profile.NewClientProfile()
	p.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	p.HttpProfile.ReqTimeout = 10
	p.NetworkFailureMaxRetries = 0
	p.RateLimitExceededMaxRetries = 0
	p.UnsafeRetryOnConnectionFailure = false
	p.DisableRegionBreaker = true
	return sms.NewClient(common.NewCredential(secretID, secretKey), region, p)
}

func tcValue[T any](p *T) (zero T) {
	if p != nil {
		return *p
	}
	return zero
}

func tencentFailure(err error, consuming bool) error {
	if err == nil {
		return nil
	}
	result := &domain.RemoteResourceError{Code: "TRANSPORT_ERROR", Message: "腾讯云请求未完成", Uncertain: consuming}
	var sdkErr *tcerr.TencentCloudSDKError
	if errors.As(err, &sdkErr) {
		result.Code = sdkErr.GetCode()
		result.RequestID = sdkErr.GetRequestId()
		if !strings.HasPrefix(result.Code, "ClientError.") {
			result.Message = "腾讯云：" + sdkErr.GetMessage()
			result.Uncertain = consuming && strings.HasPrefix(result.Code, "InternalError")
		}
	}
	return result
}

func tencentInvalidResponse(requestID string, consuming bool) error {
	return &domain.RemoteResourceError{Code: "INVALID_RESPONSE", Message: "腾讯云响应缺少有效数据，请查询确认结果", RequestID: requestID, Uncertain: consuming}
}

func tencentAudit(code *int64) int8 {
	if code == nil {
		return 0
	}
	switch *code {
	case 0:
		return 2
	case 1, 2:
		return 1
	case -1:
		return 3
	default:
		return 0
	}
}
