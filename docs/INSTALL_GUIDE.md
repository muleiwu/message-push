# 安装引导功能说明

## 概述

本项目实现了完整的 Web 安装引导功能，允许用户通过浏览器界面完成系统的初始化配置，包括数据库配置、Redis 配置和管理员账户创建。

## 功能特性

### 后端 API

提供 4 个安装相关的 API 接口（无需认证）：

1. **检查安装状态**
   - 路径: `GET /api/install/check`
   - 功能: 检查系统是否已安装、数据库连接状态等
   - 响应: 
     ```json
     {
       "code": 0,
       "message": "success",
       "data": {
         "installed": false,
         "databaseConnected": true,
         "databaseConfigured": true,
         "message": "系统未安装",
         "currentDatabase": "mysql"
       }
     }
     ```

2. **测试数据库连接**
   - 路径: `POST /api/install/test-connection`
   - 功能: 测试提供的数据库配置是否可用
   - 请求体:
     ```json
     {
       "driver": "mysql",
       "host": "localhost",
       "port": 3306,
       "username": "root",
       "password": "password",
       "database": "message_push",
       "charset": "utf8mb4"
     }
     ```
   - 支持的数据库: MySQL、PostgreSQL

3. **测试 Redis 连接**
   - 路径: `POST /api/install/test-redis`
   - 功能: 测试提供的 Redis 配置是否可用
   - 请求体:
     ```json
     {
       "host": "localhost",
       "port": 6379,
       "password": "",
       "db": 0
     }
     ```

4. **提交安装**
   - 路径: `POST /api/install/submit`
   - 功能: 完成系统安装，包括配置写入、数据库迁移、管理员创建
   - 请求体:
     ```json
     {
       "database": {
         "driver": "mysql",
         "host": "localhost",
         "port": 3306,
         "username": "root",
         "password": "password",
         "database": "message_push",
         "charset": "utf8mb4"
       },
       "redis": {
         "host": "localhost",
         "port": 6379,
         "password": "",
         "db": 0
       },
       "admin": {
         "username": "admin",
         "password": "admin123",
         "email": "admin@example.com",
         "realName": "管理员",
         "phone": "13800138000"
       }
     }
     ```

### 前端界面

前端安装界面位于 `admin-webui/apps/web-install`，提供三步安装向导：

1. **数据库配置** - 配置数据库和 Redis 连接，支持连接测试
2. **管理员账户** - 创建系统管理员账户
3. **完成安装** - 显示安装结果，跳转到登录页

## 使用流程

1. **首次启动**
   - 启动服务后，访问安装页面: `http://localhost:8080/install/`
   - 如果系统已安装，会自动跳转到管理后台登录页

2. **配置数据库**
   - 选择数据库类型（MySQL 或 PostgreSQL）
   - 填写数据库连接信息
   - 点击"测试数据库连接"确保配置正确
   - 填写 Redis 连接信息
   - 点击"测试 Redis 连接"确保配置正确
   - 两个测试都通过后，点击"下一步"

3. **创建管理员**
   - 填写管理员用户名、密码、邮箱、真实姓名
   - 手机号为可选项
   - 点击"开始安装"

4. **完成安装**
   - 系统会自动执行以下操作：
     - 将数据库配置写入 `config.yaml`
     - 将 Redis 配置写入 `config.yaml`
     - 自动创建所有数据库表
     - 创建管理员账户（密码使用 bcrypt 加密）
     - 触发配置重载
   - 安装成功后，点击"前往登录"使用管理员账户登录

## 技术实现

### 后端

- **DTO 层** (`app/dto/install_dto.go`): 定义安装相关的请求和响应结构
- **Service 层** (`app/service/install_service.go`): 实现安装业务逻辑
  - 数据库连接测试
  - Redis 连接测试
  - 配置文件更新（使用 viper）
  - 数据库表自动迁移（使用 GORM AutoMigrate）
  - 管理员账户创建（bcrypt 密码加密）
- **Controller 层** (`app/controller/install_controller.go`): 处理 HTTP 请求
- **路由注册** (`config/autoload/router.go`): 注册安装 API 路由

### 前端

- **Vue 3 + Ant Design Vue** 实现友好的安装向导界面
- **步骤指示器** 引导用户完成安装流程
- **实时验证** 在提交前验证必填字段
- **连接测试** 确保配置正确后才能进入下一步

## 安全注意事项

1. **生产环境建议**
   - 安装完成后，建议禁用或保护安装接口
   - 使用强密码作为管理员密码
   - 定期更换管理员密码

2. **配置文件**
   - `config.yaml` 包含敏感信息（数据库密码、Redis 密码）
   - 应妥善保管配置文件，不要提交到版本控制系统

3. **防止重复安装**
   - 系统通过检查 `admin_users` 表是否有记录来判断是否已安装
   - 已安装的系统无法重复执行安装流程

## 支持的数据库

- MySQL 5.7+
- PostgreSQL 9.6+

注: SQLite 暂不支持，如需添加支持，需在 `go.mod` 中添加 `gorm.io/driver/sqlite` 依赖。

