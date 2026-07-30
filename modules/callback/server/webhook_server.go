package server

import (
	"context"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/callback/infrastructure"
)

// WebhookServer runs the durable webhook outbox dispatcher.
type WebhookServer struct {
	dispatcher *infrastructure.WebhookDispatcher
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewWebhookServer() *WebhookServer {
	return &WebhookServer{}
}

func (s *WebhookServer) Run() error {
	if !helper.GetConfig().GetBool("app.installed", false) {
		helper.GetLogger().Info("系统未安装，跳过 WebhookServer 启动")
		return nil
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.dispatcher = infrastructure.NewWebhookDispatcher()
	s.dispatcher.Start(s.ctx)
	return nil
}

func (s *WebhookServer) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.dispatcher != nil {
		s.dispatcher.Stop()
	}
	return nil
}
