package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminUserService 管理员用户管理服务
type AdminUserService struct{}

// NewAdminUserService 创建管理员用户管理服务实例
func NewAdminUserService() *AdminUserService {
	return &AdminUserService{}
}

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateUser 创建管理员用户
func (s *AdminUserService) CreateUser(req *dto.CreateAdminUserRequest) (*dto.AdminUserResponse, error) {
	logger := helper.GetHelper().GetLogger()
	userDAO := dao.NewAdminUserDAO()

	// 检查用户名是否已存在
	if userDAO.UsernameExists(req.Username) {
		return nil, fmt.Errorf("用户名已存在")
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("密码加密失败: " + err.Error())
		return nil, fmt.Errorf("密码加密失败")
	}

	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}

	user := &model.AdminUser{
		Username: req.Username,
		Password: string(hashedPassword),
		RealName: req.RealName,
		Status:   status,
	}

	if err := userDAO.Create(user); err != nil {
		logger.Error("创建用户失败: " + err.Error())
		return nil, fmt.Errorf("创建用户失败")
	}

	logger.Info("管理员用户创建成功: " + user.Username)

	return &dto.AdminUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		RealName:  user.RealName,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetUserList 获取管理员用户列表
func (s *AdminUserService) GetUserList(req *dto.AdminUserListRequest) (*dto.AdminUserListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	userDAO := dao.NewAdminUserDAO()

	users, total, err := userDAO.GetList(page, pageSize, req.Username, req.Status)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}

	items := make([]*dto.AdminUserResponse, 0, len(users))
	for _, user := range users {
		items = append(items, &dto.AdminUserResponse{
			ID:        user.ID,
			Username:  user.Username,
			RealName:  user.RealName,
			Status:    user.Status,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
			UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.AdminUserListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetUserByID 获取管理员用户详情
func (s *AdminUserService) GetUserByID(id uint) (*dto.AdminUserResponse, error) {
	userDAO := dao.NewAdminUserDAO()
	user, err := userDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	return &dto.AdminUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		RealName:  user.RealName,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateUser 更新管理员用户
func (s *AdminUserService) UpdateUser(id uint, req *dto.UpdateAdminUserRequest) error {
	logger := helper.GetHelper().GetLogger()
	userDAO := dao.NewAdminUserDAO()

	// 检查用户是否存在
	user, err := userDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	// 更新字段
	if req.RealName != "" {
		user.RealName = req.RealName
	}
	if req.Status == 0 || req.Status == 1 {
		user.Status = req.Status
	}

	if err := userDAO.Update(user); err != nil {
		logger.Error("更新用户失败: " + err.Error())
		return fmt.Errorf("更新用户失败")
	}

	logger.Info("管理员用户更新成功: " + user.Username)
	return nil
}

// DeleteUser 删除管理员用户
func (s *AdminUserService) DeleteUser(id uint) error {
	logger := helper.GetHelper().GetLogger()
	userDAO := dao.NewAdminUserDAO()

	// 检查用户是否存在
	user, err := userDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}

	if err := userDAO.Delete(id); err != nil {
		logger.Error("删除用户失败: " + err.Error())
		return fmt.Errorf("删除用户失败")
	}

	logger.Info("管理员用户删除成功: " + user.Username)
	return nil
}

// ResetPassword 重置管理员用户密码
func (s *AdminUserService) ResetPassword(id uint, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	logger := helper.GetHelper().GetLogger()
	userDAO := dao.NewAdminUserDAO()

	// 检查用户是否存在
	user, err := userDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	var plainPassword string

	// 确定使用的密码
	if req.AutoGenerate || req.Password == "" {
		// 生成随机密码
		randomPwd, err := generateRandomPassword(16)
		if err != nil {
			logger.Error("生成随机密码失败: " + err.Error())
			return nil, fmt.Errorf("生成随机密码失败")
		}
		plainPassword = randomPwd
	} else {
		// 使用指定密码
		plainPassword = req.Password
	}

	// 密码加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("密码加密失败: " + err.Error())
		return nil, fmt.Errorf("密码加密失败")
	}

	// 更新密码
	if err := userDAO.UpdatePassword(id, string(hashedPassword)); err != nil {
		logger.Error("更新密码失败: " + err.Error())
		return nil, fmt.Errorf("更新密码失败")
	}

	logger.Info("管理员用户密码重置成功: " + user.Username)

	return &dto.ResetPasswordResponse{
		Password: plainPassword,
	}, nil
}
