# Admin 控制器模块

本目录包含管理后台的所有控制器，按功能模块拆分。

## 文件结构

```
app/controller/admin/
├── README.md                      # 说明文档（本文件）
├── application_controller.go      # 应用管理控制器
├── provider_controller.go         # 服务商管理控制器
├── channel_controller.go          # 通道管理控制器
└── statistics_controller.go       # 统计分析控制器
```

## 控制器说明

### 1. ApplicationController (application_controller.go)

**职责**：应用管理相关接口

**方法列表**：
- `CreateApplication` - 创建应用
- `GetApplicationList` - 获取应用列表
- `GetApplication` - 获取应用详情
- `UpdateApplication` - 更新应用
- `DeleteApplication` - 删除应用
- `RegenerateSecret` - 重新生成密钥

**API路由**：
```
POST   /api/admin/applications
GET    /api/admin/applications
GET    /api/admin/applications/:id
PUT    /api/admin/applications/:id
DELETE /api/admin/applications/:id
POST   /api/admin/applications/regenerate-secret
```

---

### 2. ProviderController (provider_controller.go)

**职责**：服务商管理相关接口

**方法列表**：
- `CreateProvider` - 创建服务商
- `GetProviderList` - 获取服务商列表
- `GetProvider` - 获取服务商详情
- `UpdateProvider` - 更新服务商
- `DeleteProvider` - 删除服务商

**API路由**：
```
POST   /api/admin/providers
GET    /api/admin/providers
GET    /api/admin/providers/:id
PUT    /api/admin/providers/:id
DELETE /api/admin/providers/:id
```

---

### 3. ChannelController (channel_controller.go)

**职责**：通道管理相关接口

**方法列表**：
- `CreateChannel` - 创建通道
- `GetChannelList` - 获取通道列表
- `GetChannel` - 获取通道详情
- `UpdateChannel` - 更新通道
- `DeleteChannel` - 删除通道
- `BindProviderToChannel` - 绑定服务商到通道

**API路由**：
```
POST   /api/admin/channels
GET    /api/admin/channels
GET    /api/admin/channels/:id
PUT    /api/admin/channels/:id
DELETE /api/admin/channels/:id
POST   /api/admin/channels/bind-provider
```

---

### 4. StatisticsController (statistics_controller.go)

**职责**：统计分析相关接口

**方法列表**：
- `GetStatistics` - 获取统计数据

**API路由**：
```
GET /api/admin/statistics
```

---

## 使用方法

### 在路由中注册

参考 `config/autoload/router.go`：

```go
import (
    "cnb.cool/mliev/push/message-push/app/controller/admin"
)

// 初始化各个控制器
appCtrl := admin.NewApplicationController()
providerCtrl := admin.NewProviderController()
channelCtrl := admin.NewChannelController()
statsCtrl := admin.NewStatisticsController()

// 注册路由
adminGroup := router.Group("/api/admin")
{
    // 应用管理
    apps := adminGroup.Group("/applications")
    {
        apps.GET("", appCtrl.GetApplicationList)
        apps.POST("", appCtrl.CreateApplication)
        // ... 其他路由
    }
    
    // ... 其他模块
}
```

---

## 设计原则

### 1. 单一职责原则
每个控制器只负责一个功能模块，便于维护和扩展。

### 2. 依赖注入
所有控制器都依赖 `AdminService`，通过构造函数注入：

```go
type ApplicationController struct {
    adminService *service.AdminService
}

func NewApplicationController() *ApplicationController {
    return &ApplicationController{
        adminService: service.NewAdminService(),
    }
}
```

### 3. 统一响应格式
所有控制器使用统一的响应函数：
- `controller.SuccessResponse(ctx, data)` - 成功响应
- `controller.ErrorResponse(ctx, code, message)` - 错误响应

### 4. 统一错误处理
- 参数验证失败：返回 400 错误
- 资源不存在：返回 404 错误
- 服务器错误：返回 500 错误

---

## 后续扩展

如需添加新的管理功能模块，可以按照以下步骤：

1. 创建新的控制器文件（如 `log_controller.go`）
2. 实现控制器结构体和方法
3. 在 `router.go` 中注册新的控制器和路由
4. 更新本 README 文档

---

## 注意事项

1. **认证授权**：当前管理API未实现认证，生产环境务必添加JWT或其他认证机制
2. **权限控制**：建议添加基于角色的权限控制（RBAC）
3. **审计日志**：建议记录所有管理操作的审计日志
4. **参数验证**：所有输入参数都会进行严格的验证
5. **软删除**：删除操作使用软删除，数据不会真正删除

---

## 版本历史

### v1.0.0 (2025-11-19)
- ✅ 将 `admin_controller.go` 拆分为4个独立的控制器文件
- ✅ 按功能模块组织代码，提高可维护性
- ✅ 更新路由配置以使用新的控制器结构
- ✅ 添加完整的文档说明

