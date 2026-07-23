package main

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func seedDemo(db *gorm.DB, anchor time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		admins, err := seedAdmins(tx, anchor)
		if err != nil {
			return err
		}
		_ = admins
		apps, err := seedApplications(tx, anchor)
		if err != nil {
			return err
		}
		accounts, err := seedProviderAccounts(tx, anchor)
		if err != nil {
			return err
		}
		templates, err := seedTemplates(tx, accounts, anchor)
		if err != nil {
			return err
		}
		channels, err := seedChannels(tx, accounts, templates, anchor)
		if err != nil {
			return err
		}
		if err := seedRules(tx, anchor); err != nil {
			return err
		}
		return seedActivity(tx, apps, accounts, channels, anchor)
	})
}

func seedAdmins(db *gorm.DB, anchor time.Time) ([]model.AdminUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("demo-pass-2026"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	emailLocal := "demo-admin@example.com"
	emailSSO := "sso-admin@example.com"
	oidcSub := "demo-oidc-subject-001"
	users := []model.AdminUser{
		{Username: "demo-admin", Password: string(hash), RealName: "演示管理员", Email: &emailLocal, AuthSource: "local", Status: 1, CreatedAt: anchor.AddDate(0, 0, -120), UpdatedAt: anchor.Add(-2 * time.Hour)},
		{Username: "sso-admin", Password: "!demo-sso-no-password!", RealName: "演示 SSO 管理员", Email: &emailSSO, OidcSub: &oidcSub, AuthSource: "oidc", Status: 1, CreatedAt: anchor.AddDate(0, 0, -80), UpdatedAt: anchor.AddDate(0, 0, -2)},
		{Username: "disabled-admin", Password: string(hash), RealName: "已停用管理员", AuthSource: "local", Status: 0, CreatedAt: anchor.AddDate(0, 0, -60), UpdatedAt: anchor.AddDate(0, 0, -10)},
	}
	if err := db.Create(&users).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&users[2]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func seedApplications(db *gorm.DB, anchor time.Time) ([]model.Application, error) {
	values := []struct {
		id, secret, name, whitelist, webhook string
		status, quota, rate                  int
	}{
		{"demo_shop", "demo-shop-secret-2026", "演示商城", "127.0.0.1\n192.0.2.0/24", "https://webhook.example.com/message-push/demo-shop", 1, 50000, 200},
		{"demo_ops", "demo-ops-secret-2026", "演示运维中心", "", "https://webhook.example.com/message-push/demo-ops", 1, 10000, 50},
		{"demo_legacy", "demo-disabled-secret-2026", "已停用旧应用", "198.51.100.8", "https://disabled.example.com/callback", 0, 1000, 10},
	}
	apps := make([]model.Application, 0, len(values))
	for i, value := range values {
		encrypted, err := appHelper.EncryptAppSecret(value.secret)
		if err != nil {
			return nil, err
		}
		apps = append(apps, model.Application{
			AppID: value.id, AppSecret: encrypted, AppName: value.name, Status: int8(value.status),
			IPWhitelist: value.whitelist, WebhookURL: value.webhook, DailyQuota: value.quota, RateLimit: value.rate,
			CreatedAt: anchor.AddDate(0, 0, -90+i*10), UpdatedAt: anchor.AddDate(0, 0, -3),
		})
	}
	if err := db.Create(&apps).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&apps[2]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func seedProviderAccounts(db *gorm.DB, anchor time.Time) ([]model.ProviderAccount, error) {
	accounts := []model.ProviderAccount{
		{AccountCode: "demo_aliyun_sms", AccountName: "阿里云短信（演示主账号）", ProviderCode: constants.ProviderAliyunSMS, ProviderType: constants.MessageTypeSMS, Status: 1, Remark: "纯演示凭证，不会发起真实请求", CreatedAt: anchor.AddDate(0, 0, -70), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{AccountCode: "demo_tencent_sms", AccountName: "腾讯云短信（演示备用）", ProviderCode: constants.ProviderTencentSMS, ProviderType: constants.MessageTypeSMS, Status: 1, Remark: "纯演示凭证，不会发起真实请求", CreatedAt: anchor.AddDate(0, 0, -69), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{AccountCode: "demo_smtp", AccountName: "SMTP 邮件（演示）", ProviderCode: constants.ProviderSMTP, ProviderType: constants.MessageTypeEmail, Status: 1, Remark: "仅用于本地页面展示", CreatedAt: anchor.AddDate(0, 0, -68), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{AccountCode: "demo_disabled_sms", AccountName: "历史短信账号（已停用）", ProviderCode: constants.ProviderAliyunSMS, ProviderType: constants.MessageTypeSMS, Status: 0, Remark: "演示停用状态", CreatedAt: anchor.AddDate(0, 0, -67), UpdatedAt: anchor.AddDate(0, 0, -20)},
	}
	configs := []map[string]interface{}{
		{"access_key_id": "LTAI5tDEMOONLY00000001", "access_key_secret": "DEMO_ONLY_NOT_A_REAL_SECRET_01"},
		{"secret_id": "AKID_DEMO_ONLY_00000002", "secret_key": "DEMO_ONLY_NOT_A_REAL_SECRET_02", "sdk_app_id": "1400000000", "region": "ap-guangzhou"},
		{"host": "smtp.example.com", "port": 587, "username": "demo-sender@example.com", "password": "DEMO_ONLY_SMTP_PASSWORD", "from": "noreply@example.com", "encryption": "starttls"},
		{"access_key_id": "LTAI5tDISABLEDDEMO0003", "access_key_secret": "DISABLED_DEMO_ONLY_SECRET_03"},
	}
	for i := range accounts {
		if err := accounts[i].SetConfig(configs[i]); err != nil {
			return nil, err
		}
	}
	if err := db.Create(&accounts).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&accounts[3]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

type demoTemplates struct {
	System   []model.MessageTemplate
	Provider []model.ProviderTemplate
	Signs    []model.ProviderSignature
}

func seedTemplates(db *gorm.DB, accounts []model.ProviderAccount, anchor time.Time) (*demoTemplates, error) {
	system := []model.MessageTemplate{
		{TemplateName: "登录验证码", ContentType: "text", Content: "您的验证码是 {code}，{minutes} 分钟内有效。", Description: "演示短信验证码模板", Status: 1, CreatedAt: anchor.AddDate(0, 0, -60), UpdatedAt: anchor.AddDate(0, 0, -5)},
		{TemplateName: "订单状态通知", ContentType: "text", Content: "订单 {order_no} 已更新为 {status}。", Description: "演示订单状态短信", Status: 1, CreatedAt: anchor.AddDate(0, 0, -58), UpdatedAt: anchor.AddDate(0, 0, -5)},
		{TemplateName: "服务告警邮件", ContentType: "html", Content: "<h2>{service} 告警</h2><p>级别：{level}</p><p>{detail}</p>", Description: "演示 HTML 邮件模板", Status: 1, CreatedAt: anchor.AddDate(0, 0, -55), UpdatedAt: anchor.AddDate(0, 0, -5)},
		{TemplateName: "历史活动通知（停用）", ContentType: "text", Content: "活动 {name} 已结束。", Description: "用于展示停用状态", Status: 0, CreatedAt: anchor.AddDate(0, 0, -100), UpdatedAt: anchor.AddDate(0, 0, -30)},
	}
	variables := [][]string{{"code", "minutes"}, {"order_no", "status"}, {"service", "level", "detail"}, {"name"}}
	for i := range system {
		if err := system[i].SetVariables(variables[i]); err != nil {
			return nil, err
		}
	}
	if err := db.Create(&system).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&system[3]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}

	providers := []model.ProviderTemplate{
		{ProviderID: accounts[0].ID, TemplateCode: "SMS_DEMO_100001", TemplateName: "阿里云登录验证码（演示）", ContentType: "text", TemplateContent: "您的验证码是 ${code}，${minutes} 分钟内有效。", Status: 1, Remark: "虚构模板代码", CreatedAt: anchor.AddDate(0, 0, -50), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderID: accounts[1].ID, TemplateCode: "200001", TemplateName: "腾讯云登录验证码（演示）", ContentType: "text", TemplateContent: "您的验证码是 {1}，{2} 分钟内有效。", Status: 1, Remark: "虚构模板代码", CreatedAt: anchor.AddDate(0, 0, -49), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderID: accounts[2].ID, TemplateCode: "SMTP_DEMO_ALERT", TemplateName: "SMTP 服务告警（演示）", ContentType: "html", TemplateContent: "<h2>{{service}} 告警</h2><p>{{level}}</p><p>{{detail}}</p>", Status: 1, Remark: "本地演示模板", CreatedAt: anchor.AddDate(0, 0, -48), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderID: accounts[0].ID, TemplateCode: "SMS_DEMO_ORDER", TemplateName: "阿里云订单通知（演示）", ContentType: "text", TemplateContent: "订单 ${order_no} 已更新为 ${status}。", Status: 1, Remark: "虚构模板代码", CreatedAt: anchor.AddDate(0, 0, -47), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderID: accounts[3].ID, TemplateCode: "SMS_DISABLED_001", TemplateName: "历史模板（停用账号）", ContentType: "text", TemplateContent: "历史演示 {name}", Status: 0, Remark: "用于展示异常状态", CreatedAt: anchor.AddDate(0, 0, -90), UpdatedAt: anchor.AddDate(0, 0, -30)},
	}
	providerVars := [][]string{{"code", "minutes"}, {"code", "minutes"}, {"service", "level", "detail"}, {"order_no", "status"}, {"name"}}
	for i := range providers {
		if err := providers[i].SetVariables(providerVars[i]); err != nil {
			return nil, err
		}
	}
	if err := db.Create(&providers).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&providers[4]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}

	signs := []model.ProviderSignature{
		{ProviderAccountID: accounts[0].ID, SignatureCode: "木雷演示", SignatureName: "木雷演示（阿里云）", Status: 1, Remark: "虚构审核通过签名", CreatedAt: anchor.AddDate(0, 0, -45), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderAccountID: accounts[1].ID, SignatureCode: "木雷演示", SignatureName: "木雷演示（腾讯云）", Status: 1, Remark: "虚构审核通过签名", CreatedAt: anchor.AddDate(0, 0, -44), UpdatedAt: anchor.AddDate(0, 0, -4)},
		{ProviderAccountID: accounts[0].ID, SignatureCode: "历史签名", SignatureName: "历史签名（停用）", Status: 0, Remark: "用于展示停用状态", CreatedAt: anchor.AddDate(0, 0, -90), UpdatedAt: anchor.AddDate(0, 0, -30)},
	}
	if err := db.Create(&signs).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&signs[2]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}
	return &demoTemplates{System: system, Provider: providers, Signs: signs}, nil
}

func seedChannels(db *gorm.DB, accounts []model.ProviderAccount, templates *demoTemplates, anchor time.Time) ([]model.Channel, error) {
	channels := []model.Channel{
		{Name: "登录验证码短信（主备就绪）", Type: constants.MessageTypeSMS, MessageTemplateID: templates.System[0].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -40), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "服务告警邮件（就绪）", Type: constants.MessageTypeEmail, MessageTemplateID: templates.System[2].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -39), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "订单通知短信（降级）", Type: constants.MessageTypeSMS, MessageTemplateID: templates.System[1].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -38), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "历史活动通知（阻塞）", Type: constants.MessageTypeSMS, MessageTemplateID: templates.System[3].ID, Status: 0, CreatedAt: anchor.AddDate(0, 0, -80), UpdatedAt: anchor.AddDate(0, 0, -25)},
	}
	if err := db.Create(&channels).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&channels[3]).UpdateColumn("status", 0).Error; err != nil {
		return nil, err
	}

	bindings := []model.ChannelTemplateBinding{
		{ChannelID: channels[0].ID, ProviderTemplateID: templates.Provider[0].ID, ProviderID: accounts[0].ID, Weight: 70, Priority: 10, Status: 1, IsActive: 1, AutoDisableOnFail: true, AutoDisableThreshold: 5, CreatedAt: anchor.AddDate(0, 0, -35), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[0].ID, ProviderTemplateID: templates.Provider[1].ID, ProviderID: accounts[1].ID, Weight: 30, Priority: 10, Status: 1, IsActive: 1, AutoDisableOnFail: true, AutoDisableThreshold: 3, CreatedAt: anchor.AddDate(0, 0, -35), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[1].ID, ProviderTemplateID: templates.Provider[2].ID, ProviderID: accounts[2].ID, Weight: 100, Priority: 10, Status: 1, IsActive: 1, AutoDisableOnFail: false, AutoDisableThreshold: 5, CreatedAt: anchor.AddDate(0, 0, -34), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[2].ID, ProviderTemplateID: templates.Provider[3].ID, ProviderID: accounts[0].ID, Weight: 80, Priority: 10, Status: 1, IsActive: 1, AutoDisableOnFail: true, AutoDisableThreshold: 5, CreatedAt: anchor.AddDate(0, 0, -33), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[2].ID, ProviderTemplateID: templates.Provider[4].ID, ProviderID: accounts[3].ID, Weight: 20, Priority: 20, Status: 1, IsActive: 0, AutoDisableOnFail: true, AutoDisableThreshold: 2, CreatedAt: anchor.AddDate(0, 0, -33), UpdatedAt: anchor.AddDate(0, 0, -1)},
	}
	mappings := [][]model.ParamMappingItem{
		{{Type: model.ParamMappingTypeMapping, ProviderVar: "code", SystemVar: "code"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "minutes", SystemVar: "minutes"}},
		{{Type: model.ParamMappingTypeMapping, ProviderVar: "code", SystemVar: "code"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "minutes", SystemVar: "minutes"}},
		{{Type: model.ParamMappingTypeMapping, ProviderVar: "service", SystemVar: "service"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "level", SystemVar: "level"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "detail", SystemVar: "detail"}},
		{{Type: model.ParamMappingTypeMapping, ProviderVar: "order_no", SystemVar: "order_no"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "status", SystemVar: "status"}},
		{{Type: model.ParamMappingTypeMapping, ProviderVar: "name", SystemVar: "order_no"}},
	}
	for i := range bindings {
		if err := bindings[i].SetParamMapping(mappings[i]); err != nil {
			return nil, err
		}
	}
	if err := db.Create(&bindings).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&bindings[4]).UpdateColumn("is_active", 0).Error; err != nil {
		return nil, err
	}

	signatureMappings := []model.ChannelSignatureMapping{
		{ChannelID: channels[0].ID, SignatureName: "木雷演示", ProviderSignatureID: templates.Signs[0].ID, ProviderID: accounts[0].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -32), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[0].ID, SignatureName: "木雷演示", ProviderSignatureID: templates.Signs[1].ID, ProviderID: accounts[1].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -32), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{ChannelID: channels[2].ID, SignatureName: "木雷演示", ProviderSignatureID: templates.Signs[0].ID, ProviderID: accounts[0].ID, Status: 1, CreatedAt: anchor.AddDate(0, 0, -31), UpdatedAt: anchor.AddDate(0, 0, -3)},
	}
	if err := db.Create(&signatureMappings).Error; err != nil {
		return nil, err
	}

	health := []model.ChannelHealthHistory{
		{ProviderChannelID: bindings[0].ID, CheckTime: anchor.Add(-20 * time.Minute), Status: "healthy", ResponseTime: 128, ErrorCount: 0, SuccessRate: 99.8, IsAvailable: 1, CreatedAt: anchor.Add(-20 * time.Minute)},
		{ProviderChannelID: bindings[1].ID, CheckTime: anchor.Add(-18 * time.Minute), Status: "healthy", ResponseTime: 156, ErrorCount: 1, SuccessRate: 98.6, IsAvailable: 1, CreatedAt: anchor.Add(-18 * time.Minute)},
		{ProviderChannelID: bindings[4].ID, CheckTime: anchor.Add(-15 * time.Minute), Status: "unhealthy", ResponseTime: 920, ErrorCount: 6, SuccessRate: 72.5, IsAvailable: 0, CreatedAt: anchor.Add(-15 * time.Minute)},
	}
	return channels, db.Create(&health).Error
}

func seedRules(db *gorm.DB, anchor time.Time) error {
	rules := []model.FailureRule{
		{Name: "网络抖动自动重试", Scene: model.RuleSceneSendFailure, MessageType: constants.MessageTypeSMS, ErrorCode: "TIMEOUT,NETWORK_ERROR", ErrorKeyword: "timeout", Action: model.RuleActionRetry, ActionConfig: `{"max_retry":3,"delay_seconds":2,"backoff_rate":2,"max_delay":60}`, Priority: 100, Status: 1, Remark: "演示指数退避", CreatedAt: anchor.AddDate(0, 0, -30), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "阿里云失败切换腾讯云", Scene: model.RuleSceneSendFailure, ProviderCode: constants.ProviderAliyunSMS, MessageType: constants.MessageTypeSMS, ErrorCode: "ISP.SYSTEM_ERROR", Action: model.RuleActionSwitchProvider, ActionConfig: `{"exclude_current":true,"max_retry":1}`, Priority: 90, Status: 1, Remark: "演示主备切换", CreatedAt: anchor.AddDate(0, 0, -29), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "回执失败告警", Scene: model.RuleSceneCallbackFailure, MessageType: constants.MessageTypeSMS, ErrorKeyword: "rejected", Action: model.RuleActionAlert, ActionConfig: `{"webhook_url":"https://alerts.example.com/message-push/demo","alert_level":"warning"}`, Priority: 80, Status: 1, Remark: "演示地址，不会真实调用", CreatedAt: anchor.AddDate(0, 0, -28), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{Name: "历史直接失败规则", Scene: model.RuleSceneSendFailure, Action: model.RuleActionFail, ActionConfig: `{}`, Priority: 10, Status: 0, Remark: "用于展示停用状态", CreatedAt: anchor.AddDate(0, 0, -80), UpdatedAt: anchor.AddDate(0, 0, -20)},
	}
	if err := db.Create(&rules).Error; err != nil {
		return err
	}
	return db.Model(&rules[3]).UpdateColumn("status", 0).Error
}

func seedActivity(db *gorm.DB, apps []model.Application, accounts []model.ProviderAccount, channels []model.Channel, anchor time.Time) error {
	webhooks := []model.WebhookConfig{
		{AppID: apps[0].AppID, WebhookURL: "https://webhook.example.com/message-push/demo-shop", Secret: "DEMO_WEBHOOK_SECRET", Events: "success,failed,delivered,upstream", Status: 1, RetryCount: 3, Timeout: 5, Description: "纯演示 Webhook", CreatedAt: anchor.AddDate(0, 0, -70), UpdatedAt: anchor.AddDate(0, 0, -3)},
		{AppID: apps[1].AppID, WebhookURL: "https://webhook.example.com/message-push/demo-ops", Secret: "DEMO_OPS_WEBHOOK_SECRET", Events: "failed", Status: 0, RetryCount: 1, Timeout: 3, Description: "停用的演示 Webhook", CreatedAt: anchor.AddDate(0, 0, -60), UpdatedAt: anchor.AddDate(0, 0, -10)},
	}
	if err := db.Create(&webhooks).Error; err != nil {
		return err
	}
	if err := db.Model(&webhooks[1]).UpdateColumn("status", 0).Error; err != nil {
		return err
	}

	batches := []model.PushBatchTask{
		{BatchID: "b0000000-0000-4000-8000-000000000001", AppID: apps[0].AppID, TotalCount: 12, SuccessCount: 11, FailedCount: 1, PendingCount: 0, Status: constants.BatchStatusCompleted, CreatedAt: anchor.AddDate(0, 0, -2), UpdatedAt: anchor.AddDate(0, 0, -2).Add(8 * time.Minute)},
		{BatchID: "b0000000-0000-4000-8000-000000000002", AppID: apps[0].AppID, TotalCount: 9, SuccessCount: 6, FailedCount: 1, PendingCount: 2, Status: constants.BatchStatusProcessing, CreatedAt: anchor.Add(-90 * time.Minute), UpdatedAt: anchor.Add(-20 * time.Minute)},
		{BatchID: "b0000000-0000-4000-8000-000000000003", AppID: apps[1].AppID, TotalCount: 5, SuccessCount: 0, FailedCount: 5, PendingCount: 0, Status: constants.BatchStatusFailed, CreatedAt: anchor.AddDate(0, 0, -8), UpdatedAt: anchor.AddDate(0, 0, -8).Add(4 * time.Minute)},
	}
	if err := db.Create(&batches).Error; err != nil {
		return err
	}

	tasks := make([]model.PushTask, 0, 100)
	batchByTask := make(map[string]string)
	for day := 29; day >= 0; day-- {
		for n := 0; n < 3; n++ {
			seq := (29-day)*3 + n + 1
			status := constants.TaskStatusSuccess
			callbackStatus := constants.CallbackStatusDelivered
			if seq%7 == 0 {
				status = constants.TaskStatusFailed
				callbackStatus = constants.CallbackStatusFailed
			} else if seq%11 == 0 {
				status = constants.TaskStatusSent
				callbackStatus = "pending"
			}
			app := apps[seq%2]
			channel := channels[0]
			messageType := constants.MessageTypeSMS
			receiver := fmt.Sprintf("+861380000%04d", seq)
			templateParams := fmt.Sprintf(`{"code":"%06d","minutes":"5"}`, 100000+seq)
			signature := "木雷演示"
			if seq%5 == 0 {
				channel = channels[1]
				messageType = constants.MessageTypeEmail
				receiver = fmt.Sprintf("user%02d@example.com", seq)
				templateParams = `{"service":"演示订单服务","level":"warning","detail":"这是一条本地演示告警"}`
				signature = "演示服务告警"
			}
			created := anchor.AddDate(0, 0, -day).Add(time.Duration(n-2) * time.Hour)
			var callbackTime *time.Time
			if status == constants.TaskStatusSuccess || status == constants.TaskStatusFailed {
				value := created.Add(8 * time.Second)
				callbackTime = &value
			}
			taskID := fmt.Sprintf("10000000-0000-4000-8000-%012d", seq)
			tasks = append(tasks, model.PushTask{
				TaskID: taskID, AppID: app.AppID, ChannelID: channel.ID, MessageType: messageType, Receiver: receiver,
				TemplateCode: "", TemplateParams: templateParams, Signature: signature, Status: status,
				CallbackStatus: callbackStatus, CallbackTime: callbackTime, RetryCount: seq % 3, MaxRetry: 3,
				ExcludeProviderIDs: "[]", CreatedAt: created, UpdatedAt: created.Add(10 * time.Second),
			})
			if seq <= 10 || seq == 12 || seq == 13 {
				batchByTask[taskID] = batches[0].BatchID
			} else if seq > 82 && seq <= 90 {
				batchByTask[taskID] = batches[1].BatchID
			}
		}
	}

	scheduledAt := anchor.Add(24 * time.Hour)
	pendingID := "10000000-0000-4000-8000-000000000091"
	tasks = append(tasks,
		model.PushTask{TaskID: pendingID, AppID: apps[0].AppID, ChannelID: channels[0].ID, MessageType: constants.MessageTypeSMS, Receiver: "+8613800009991", TemplateParams: `{"code":"888888","minutes":"5"}`, Signature: "木雷演示", Status: constants.TaskStatusPending, CallbackStatus: "pending", MaxRetry: 3, ExcludeProviderIDs: "[]", ScheduledAt: &scheduledAt, CreatedAt: anchor.Add(-30 * time.Minute), UpdatedAt: anchor.Add(-30 * time.Minute)},
		model.PushTask{TaskID: "10000000-0000-4000-8000-000000000092", AppID: apps[1].AppID, ChannelID: channels[1].ID, MessageType: constants.MessageTypeEmail, Receiver: "pending@example.com", TemplateParams: `{"service":"演示服务","level":"info","detail":"等待队列处理"}`, Signature: "演示通知", Status: constants.TaskStatusProcessing, CallbackStatus: "pending", MaxRetry: 3, ExcludeProviderIDs: "[]", CreatedAt: anchor.Add(-15 * time.Minute), UpdatedAt: anchor.Add(-10 * time.Minute)},
	)
	batchByTask[pendingID] = batches[1].BatchID

	// 成功的管理员测试必须晚于所有通道配置更新时间，首页才会展示已验证。
	adminTestTime := anchor.Add(-5 * time.Minute)
	tasks = append(tasks, model.PushTask{TaskID: "a0000000-0000-4000-8000-000000000001", AppID: "admin_test", ChannelID: channels[0].ID, MessageType: constants.MessageTypeSMS, Receiver: "+8613800000000", TemplateParams: `{"code":"123456","minutes":"5"}`, Signature: "木雷演示", Status: constants.TaskStatusSuccess, CallbackStatus: constants.CallbackStatusDelivered, MaxRetry: 3, ExcludeProviderIDs: "[]", CreatedAt: adminTestTime, UpdatedAt: adminTestTime.Add(3 * time.Second)})
	if err := db.Create(&tasks).Error; err != nil {
		return err
	}
	for taskID, batchID := range batchByTask {
		if err := db.Exec("UPDATE push_tasks SET batch_id = ? WHERE task_id = ?", batchID, taskID).Error; err != nil {
			return err
		}
	}

	logs := make([]model.PushLog, 0, len(tasks))
	callbacks := make([]model.CallbackLog, 0, len(tasks)/2)
	webhookLogs := make([]model.WebhookLog, 0, len(tasks)/3)
	for i, task := range tasks {
		if task.Status == constants.TaskStatusPending || task.Status == constants.TaskStatusProcessing {
			continue
		}
		providerID := accounts[0].ID
		providerCode := constants.ProviderAliyunSMS
		if task.MessageType == constants.MessageTypeEmail {
			providerID = accounts[2].ID
			providerCode = constants.ProviderSMTP
		} else if i%3 == 0 {
			providerID = accounts[1].ID
			providerCode = constants.ProviderTencentSMS
		}
		logStatus := "success"
		errorMessage := ""
		response := fmt.Sprintf(`{"code":"OK","message_id":"DEMO-MSG-%04d","notice":"fake response"}`, i+1)
		if task.Status == constants.TaskStatusFailed {
			logStatus = "failed"
			errorMessage = "DEMO_PROVIDER_REJECTED：演示失败，不代表真实服务商响应"
			response = `{"code":"DEMO_REJECTED","message":"fake failure response"}`
		}
		logs = append(logs, model.PushLog{TaskID: task.TaskID, AppID: task.AppID, ProviderAccountID: providerID, ProviderMsgID: fmt.Sprintf("DEMO-MSG-%04d", i+1), RequestData: fmt.Sprintf(`{"receiver":"%s","demo":true}`, task.Receiver), ResponseData: response, Status: logStatus, ErrorMessage: errorMessage, CostTime: 80 + i%220, CreatedAt: task.CreatedAt.Add(2 * time.Second)})
		if task.CallbackStatus == constants.CallbackStatusDelivered || task.CallbackStatus == constants.CallbackStatusFailed {
			callbacks = append(callbacks, model.CallbackLog{Type: constants.CallbackTypeReport, TaskID: task.TaskID, AppID: task.AppID, ProviderCode: providerCode, ProviderID: fmt.Sprintf("DEMO-MSG-%04d", i+1), Mobile: task.Receiver, CallbackStatus: task.CallbackStatus, ErrorCode: func() string {
				if task.CallbackStatus == constants.CallbackStatusFailed {
					return "DEMO_REJECTED"
				}
				return ""
			}(), ErrorMessage: errorMessage, RawData: `{"demo":true,"source":"local-seed"}`, CreatedAt: task.CreatedAt.Add(8 * time.Second)})
		}
		if task.AppID != "admin_test" && i%3 == 0 {
			webhookLogs = append(webhookLogs, model.WebhookLog{TaskID: task.TaskID, AppID: task.AppID, WebhookConfigID: webhooks[0].ID, WebhookURL: "https://webhook.example.com/message-push/demo-shop", Event: task.Status, RequestData: `{"demo":true,"event":"delivery"}`, ResponseStatus: 200, ResponseData: `{"ok":true,"demo":true}`, Status: "success", RetryCount: 0, CreatedAt: task.CreatedAt.Add(10 * time.Second)})
		}
	}
	if err := db.CreateInBatches(&logs, 100).Error; err != nil {
		return err
	}
	if err := db.CreateInBatches(&callbacks, 100).Error; err != nil {
		return err
	}
	if err := db.CreateInBatches(&webhookLogs, 100).Error; err != nil {
		return err
	}

	upstream := []model.CallbackLog{
		{Type: constants.CallbackTypeUpstream, AppID: apps[0].AppID, ProviderCode: constants.ProviderAliyunSMS, Mobile: "+8613800001001", Content: "收到，谢谢", RawData: `{"demo":true,"content":"收到，谢谢"}`, CreatedAt: anchor.Add(-45 * time.Minute)},
		{Type: constants.CallbackTypeUpstream, AppID: apps[0].AppID, ProviderCode: constants.ProviderTencentSMS, Mobile: "+8613800001002", Content: "退订", RawData: `{"demo":true,"content":"退订"}`, CreatedAt: anchor.AddDate(0, 0, -1).Add(2 * time.Hour)},
		{Type: constants.CallbackTypeUpstream, AppID: apps[1].AppID, ProviderCode: constants.ProviderAliyunSMS, Mobile: "+8613800001003", Content: "人工客服", RawData: `{"demo":true,"content":"人工客服"}`, CreatedAt: anchor.AddDate(0, 0, -3)},
	}
	if err := db.Create(&upstream).Error; err != nil {
		return err
	}

	appStats := make([]model.AppQuotaStat, 0, 60)
	providerStats := make([]model.ProviderQuotaStat, 0, 60)
	for day := 29; day >= 0; day-- {
		date := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location()).AddDate(0, 0, -day)
		for i, app := range apps[:2] {
			total := 80 + (29-day)*7 + i*19
			failed := 2 + (day+i)%7
			appStats = append(appStats, model.AppQuotaStat{AppID: app.AppID, StatDate: date, TotalCount: total, SuccessCount: total - failed, FailedCount: failed, CreatedAt: date.Add(23 * time.Hour), UpdatedAt: date.Add(23 * time.Hour)})
		}
		for i := 0; i < 2; i++ {
			total := 60 + (29-day)*5 + i*13
			failed := 1 + (day+i)%5
			providerStats = append(providerStats, model.ProviderQuotaStat{ProviderChannelID: uint(i + 1), StatDate: date, TotalCount: total, SuccessCount: total - failed, FailedCount: failed, CreatedAt: date.Add(23 * time.Hour), UpdatedAt: date.Add(23 * time.Hour)})
		}
	}
	if err := db.CreateInBatches(&appStats, 100).Error; err != nil {
		return err
	}
	return db.CreateInBatches(&providerStats, 100).Error
}
