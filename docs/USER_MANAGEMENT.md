# 用户管理功能说明

## 功能概述

本系统新增了完整的管理员用户管理功能，支持对系统管理员用户进行 CRUD 操作和密码管理。

## 功能特性

### 1. 用户列表
- 分页显示所有管理员用户
- 支持按用户名搜索
- 支持按状态（启用/禁用）筛选
- 显示用户 ID、用户名、真实姓名、状态、创建时间、更新时间

### 2. 创建用户
- 设置用户名（3-50个字符）
- 设置密码（6-32个字符）
- 设置真实姓名（2-100个字符）
- 设置状态（启用/禁用）

### 3. 编辑用户
- 修改真实姓名
- 修改状态（启用/禁用）
- 注意：用户名创建后不可修改

### 4. 删除用户
- 软删除机制，删除的用户不会从数据库中物理删除
- 删除前需要确认

### 5. 重置密码
支持两种方式重置用户密码：

#### 方式一：自动生成随机密码（推荐）
- 系统自动生成 16 位随机密码
- 密码在重置后显示一次，请务必保存

#### 方式二：手动设置密码
- 管理员手动输入新密码
- 密码长度要求：6-32个字符
- 密码在重置后显示一次，请务必保存

### 6. 状态管理
- 通过开关快速启用/禁用用户
- 禁用的用户无法登录系统

## API 接口

### 后端路由

所有接口都在 `/api/admin/users` 路径下，需要管理员 JWT 认证。

```
GET    /api/admin/users              # 获取用户列表
POST   /api/admin/users              # 创建用户
GET    /api/admin/users/:id          # 获取用户详情
PUT    /api/admin/users/:id          # 更新用户
DELETE /api/admin/users/:id          # 删除用户
POST   /api/admin/users/:id/reset-password  # 重置密码
```

### 请求示例

#### 创建用户
```json
POST /api/admin/users
{
  "username": "admin001",
  "password": "password123",
  "real_name": "管理员001",
  "status": 1
}
```

#### 更新用户
```json
PUT /api/admin/users/1
{
  "real_name": "新的真实姓名",
  "status": 1
}
```

#### 重置密码（自动生成）
```json
POST /api/admin/users/1/reset-password
{
  "auto_generate": true
}
```

#### 重置密码（手动设置）
```json
POST /api/admin/users/1/reset-password
{
  "password": "newpassword123"
}
```

## 前端使用

### 访问路径
- 路由：`/users`
- 菜单名称：用户管理
- 图标：lucide:users

### 页面操作

1. **搜索用户**
   - 在搜索框中输入用户名
   - 选择状态筛选
   - 点击"搜索"按钮

2. **创建用户**
   - 点击"创建用户"按钮
   - 填写表单信息
   - 点击"确定"提交

3. **编辑用户**
   - 点击用户行的"编辑"按钮
   - 修改信息
   - 点击"确定"保存

4. **重置密码**
   - 点击用户行的"重置密码"按钮
   - 选择重置方式（自动生成或手动设置）
   - 点击"确定"
   - 在弹出的窗口中复制并保存新密码

5. **删除用户**
   - 点击用户行的"删除"按钮
   - 确认删除操作

6. **快速切换状态**
   - 直接点击状态列的开关按钮

## 安全提示

1. **密码安全**
   - 创建用户时请使用强密码
   - 重置密码后请立即保存，关闭窗口后无法再次查看
   - 建议定期更新密码

2. **用户管理**
   - 不要随意删除用户
   - 对于不再使用的用户，建议禁用而不是删除
   - 谨慎分配用户权限

3. **操作日志**
   - 系统会记录所有用户管理操作
   - 重要操作请确认后再执行

## 技术栈

### 后端
- Go 1.x
- Gin Web Framework
- GORM (数据库 ORM)
- bcrypt (密码加密)

### 前端
- Vue 3
- TypeScript
- Ant Design Vue
- Vben Admin

## 文件清单

### 后端文件
- `app/dto/admin_auth_dto.go` - 数据传输对象定义
- `app/dao/admin_user_dao.go` - 数据访问层
- `app/service/admin_user_service.go` - 业务逻辑层
- `app/controller/admin/admin_user_controller.go` - 控制器层
- `config/autoload/router.go` - 路由配置

### 前端文件
- `admin-webui/apps/web-antd/src/api/message-push/types.ts` - 类型定义
- `admin-webui/apps/web-antd/src/api/message-push/admin-user.ts` - API 封装
- `admin-webui/apps/web-antd/src/views/users/index.vue` - 用户管理页面
- `admin-webui/apps/web-antd/src/router/routes/modules/users.ts` - 路由配置

## 测试建议

1. 创建测试用户，验证表单验证功能
2. 测试用户名重复的情况
3. 测试密码重置的两种方式
4. 测试用户状态切换
5. 测试搜索和筛选功能
6. 测试分页功能
7. 测试删除用户功能

