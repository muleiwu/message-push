package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

var _ domain.SMSReporter = (*TencentSMSSender)(nil)

func (s *TencentSMSSender) clientForEvents(req *domain.SMSEventRequest) (*sms.Client, string, error) {
	if req == nil || req.Account == nil {
		return nil, "", fmt.Errorf("供应商账号不能为空")
	}
	if req.Kind != domain.SMSReports && req.Kind != domain.SMSReplies {
		return nil, "", fmt.Errorf("不支持的短信记录类型")
	}
	if req.Limit < 1 || req.Limit > 100 {
		return nil, "", fmt.Errorf("拉取条数必须为 1～100")
	}
	config, err := req.Account.GetConfig()
	if err != nil {
		return nil, "", fmt.Errorf("腾讯云账号配置无效")
	}
	appID, _ := config["sdk_app_id"].(string)
	if appID == "" {
		return nil, "", fmt.Errorf("请先配置腾讯云 SmsSdkAppId")
	}
	factory := s.clientFactory
	if factory == nil {
		factory = newTencentClient
	}
	c, err := factory(req.Account)
	return c, appID, err
}

func (s *TencentSMSSender) QuerySMSEvents(ctx context.Context, req *domain.SMSEventRequest) (*domain.SMSEventResponse, error) {
	c, appID, err := s.clientForEvents(req)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	end := req.EndTime
	if end.IsZero() {
		end = now
	}
	if req.BeginTime.IsZero() || req.BeginTime.Before(now.Add(-7*24*time.Hour).Truncate(time.Second)) || req.BeginTime.After(end) || end.After(now.Add(time.Second)) {
		return nil, fmt.Errorf("查询时间须在最近七天内，且开始时间不得晚于结束时间")
	}
	phone := apphelper.ParsePhoneNumber(req.PhoneNumber)
	if !phone.Valid || phone.CountryCode != "86" {
		return nil, fmt.Errorf("请填写有效的中国大陆手机号码")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Kind == domain.SMSReports {
		r := sms.NewPullSmsSendStatusByPhoneNumberRequest()
		r.SmsSdkAppId, r.PhoneNumber, r.BeginTime, r.EndTime, r.Offset, r.Limit = &appID, &phone.E164, common.Uint64Ptr(uint64(req.BeginTime.Unix())), common.Uint64Ptr(uint64(end.Unix())), common.Uint64Ptr(0), common.Uint64Ptr(uint64(req.Limit))
		resp, err := c.PullSmsSendStatusByPhoneNumberWithContext(ctx, r)
		if err != nil {
			return nil, tencentFailure(err, false)
		}
		if resp == nil || resp.Response == nil {
			return nil, tencentInvalidResponse("", false)
		}
		return tencentReportEvents(resp.Response.PullSmsSendStatusSet, tcValue(resp.Response.RequestId), req.Limit), nil
	}
	r := sms.NewPullSmsReplyStatusByPhoneNumberRequest()
	r.SmsSdkAppId, r.PhoneNumber, r.BeginTime, r.EndTime, r.Offset, r.Limit = &appID, &phone.E164, common.Uint64Ptr(uint64(req.BeginTime.Unix())), common.Uint64Ptr(uint64(end.Unix())), common.Uint64Ptr(0), common.Uint64Ptr(uint64(req.Limit))
	resp, err := c.PullSmsReplyStatusByPhoneNumberWithContext(ctx, r)
	if err != nil {
		return nil, tencentFailure(err, false)
	}
	if resp == nil || resp.Response == nil {
		return nil, tencentInvalidResponse("", false)
	}
	return tencentReplyEvents(resp.Response.PullSmsReplyStatusSet, tcValue(resp.Response.RequestId), req.Limit), nil
}

func (s *TencentSMSSender) PullSMSEvents(ctx context.Context, req *domain.SMSEventRequest) (*domain.SMSEventResponse, error) {
	c, appID, err := s.clientForEvents(req)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Kind == domain.SMSReports {
		r := sms.NewPullSmsSendStatusRequest()
		r.SmsSdkAppId, r.Limit = &appID, common.Uint64Ptr(uint64(req.Limit))
		resp, err := c.PullSmsSendStatusWithContext(ctx, r)
		if err != nil {
			return nil, tencentFailure(err, true)
		}
		if resp == nil || resp.Response == nil {
			return nil, tencentInvalidResponse("", true)
		}
		return tencentReportEvents(resp.Response.PullSmsSendStatusSet, tcValue(resp.Response.RequestId), req.Limit), nil
	}
	r := sms.NewPullSmsReplyStatusRequest()
	r.SmsSdkAppId, r.Limit = &appID, common.Uint64Ptr(uint64(req.Limit))
	resp, err := c.PullSmsReplyStatusWithContext(ctx, r)
	if err != nil {
		return nil, tencentFailure(err, true)
	}
	if resp == nil || resp.Response == nil {
		return nil, tencentInvalidResponse("", true)
	}
	return tencentReplyEvents(resp.Response.PullSmsReplyStatusSet, tcValue(resp.Response.RequestId), req.Limit), nil
}

func tencentEventPhone(full, country, subscriber *string) string {
	value := tcValue(full)
	if value == "" {
		value = "+" + tcValue(country) + tcValue(subscriber)
	}
	parsed := apphelper.ParsePhoneNumber(value)
	if parsed.Valid {
		return parsed.E164
	}
	return value
}

func tencentReportStatus(status string) string {
	switch status {
	case "SUCCESS":
		return constants.CallbackStatusDelivered
	case "FAIL":
		return constants.CallbackStatusFailed
	default:
		return "unknown"
	}
}

func tencentReportEvents(rows []*sms.PullSmsSendStatus, requestID string, limit int) *domain.SMSEventResponse {
	result := &domain.SMSEventResponse{Items: []domain.SMSEvent{}, RequestID: requestID, LimitReached: len(rows) >= limit}
	for _, row := range rows {
		raw, _ := json.Marshal(row)
		event := domain.SMSEvent{Status: "unknown", RawData: string(raw)}
		if row != nil {
			event.ProviderMsgID, event.Mobile, event.Status = tcValue(row.SerialNo), tencentEventPhone(row.PhoneNumber, row.CountryCode, row.SubscriberNumber), tencentReportStatus(tcValue(row.ReportStatus))
			event.ErrorMessage = tcValue(row.Description)
			if tcValue(row.UserReceiveTime) > 0 {
				event.OccurredAt = time.Unix(int64(*row.UserReceiveTime), 0).UTC()
			}
		}
		result.Items = append(result.Items, event)
	}
	return result
}

func tencentReplyEvents(rows []*sms.PullSmsReplyStatus, requestID string, limit int) *domain.SMSEventResponse {
	result := &domain.SMSEventResponse{Items: []domain.SMSEvent{}, RequestID: requestID, LimitReached: len(rows) >= limit}
	for _, row := range rows {
		raw, _ := json.Marshal(row)
		event := domain.SMSEvent{RawData: string(raw)}
		if row != nil {
			event.Mobile, event.Content, event.SignName, event.ExtendCode = tencentEventPhone(row.PhoneNumber, row.CountryCode, row.SubscriberNumber), tcValue(row.ReplyContent), tcValue(row.SignName), tcValue(row.ExtendCode)
			if tcValue(row.ReplyTime) > 0 {
				event.OccurredAt = time.Unix(int64(*row.ReplyTime), 0).UTC()
			}
		}
		result.Items = append(result.Items, event)
	}
	return result
}

func (s *TencentSMSSender) SupportsStatusPull() bool { return true }

func (s *TencentSMSSender) PullStatus(ctx context.Context, req *domain.StatusPullRequest) (*domain.StatusQueryResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("拉取参数不能为空")
	}
	resp, err := s.PullSMSEvents(ctx, &domain.SMSEventRequest{Account: req.ProviderAccount, Kind: domain.SMSReports, Limit: 100})
	if err != nil {
		return nil, err
	}
	return tencentLegacyReports(resp, ""), nil
}

func tencentLegacyReports(resp *domain.SMSEventResponse, messageID string) *domain.StatusQueryResponse {
	result := &domain.StatusQueryResponse{Results: []*domain.StatusQueryResult{}, RequestID: resp.RequestID}
	for _, row := range resp.Items {
		if messageID != "" && row.ProviderMsgID != messageID {
			continue
		}
		result.Results = append(result.Results, &domain.StatusQueryResult{ProviderMsgID: row.ProviderMsgID, PhoneNumber: row.Mobile, Status: row.Status, ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, ReportTime: row.OccurredAt})
	}
	return result
}
