// Package domain 是 messaging 限界上下文的领域层：
// 定义消息应用服务端口（Service）——推送任务的创建、发送、批量发送与查询。
// PushTask 聚合根目前位于共享内核 app/model。
package domain

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
)

// Service 消息应用服务端口：编排通道选择（channel）、任务入队（delivery）、
// 模板渲染与配额校验，完成推送任务的创建与查询。
// 基础设施层提供实现，经 assembly 注册到 go-web 容器，供 controller / admin 等调用。
type Service interface {
	// Send 创建并入队单条推送任务
	Send(ctx context.Context, req *dto.SendRequest) (*dto.SendResponse, error)
	// BatchSend 创建并入队批量推送任务
	BatchSend(ctx context.Context, req *dto.BatchSendRequest) (*dto.BatchSendResponse, error)
	// QueryTask 按任务 ID 查询推送任务
	QueryTask(ctx context.Context, taskID string) (*model.PushTask, error)
}
