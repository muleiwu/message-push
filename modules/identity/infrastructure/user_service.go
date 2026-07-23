package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"github.com/muleiwu/gsr"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	adminAuthSourceLocal = "local"
	adminAuthSourceOIDC  = "oidc"
)

// 确保实现 domain.UserService 端口。
var _ domain.UserService = (*AdminUserService)(nil)

// adminUserCRUDStore 隔离业务规则与 GORM，便于对邮箱和身份规则做精确单测。
type adminUserCRUDStore interface {
	Create(user *model.AdminUser) error
	Delete(id uint) error
	EmailExists(email string, excludeID uint) (bool, error)
	GetByID(id uint) (*model.AdminUser, error)
	GetList(page, pageSize int, username string, status *int8) ([]*model.AdminUser, int64, error)
	Update(user *model.AdminUser) error
	UpdatePassword(id uint, hashedPassword string) error
	UsernameExists(username string) bool
}

// AdminUserService 管理员用户管理服务。
type AdminUserService struct {
	store  adminUserCRUDStore
	logger gsr.Logger
}

// NewAdminUserService 创建管理员用户管理服务实例。
func NewAdminUserService() *AdminUserService {
	return &AdminUserService{
		store:  dao.NewAdminUserDAO(),
		logger: helper.GetLogger(),
	}
}

func newAdminUserService(store adminUserCRUDStore) *AdminUserService {
	return &AdminUserService{store: store}
}

// generateRandomPassword 生成随机密码。
func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateUser 创建本地管理员用户。
func (s *AdminUserService) CreateUser(req *dto.CreateAdminUserRequest) (*dto.AdminUserResponse, error) {
	if s.store.UsernameExists(req.Username) {
		return nil, domain.ErrAdminUsernameConflict
	}

	email, err := appHelper.NormalizeAdminEmail(req.Email)
	if err != nil || email == "" {
		return nil, domain.ErrInvalidAdminEmail
	}
	emailExists, err := s.store.EmailExists(email, 0)
	if err != nil {
		return nil, fmt.Errorf("检查邮箱失败: %w", err)
	}
	if emailExists {
		return nil, domain.ErrAdminEmailConflict
	}

	status, err := normalizeAdminStatus(req.Status, 1)
	if err != nil {
		return nil, err
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logError("密码加密失败: " + err.Error())
		return nil, fmt.Errorf("密码加密失败")
	}

	user := &model.AdminUser{
		Username:   req.Username,
		Password:   string(hashedPassword),
		RealName:   req.RealName,
		Email:      &email,
		AuthSource: adminAuthSourceLocal,
		Status:     status,
	}

	if err := s.store.Create(user); err != nil {
		s.logError("创建用户失败: " + err.Error())
		if isDuplicateKeyError(err) {
			// TranslateError 可能丢失索引名；并发写入失败后再查一次邮箱，
			// 确保邮箱冲突不会被误报为用户名冲突。
			emailExists, existsErr := s.store.EmailExists(email, 0)
			if existsErr == nil && emailExists {
				return nil, domain.ErrAdminEmailConflict
			}
			if strings.Contains(strings.ToLower(err.Error()), "email") {
				return nil, domain.ErrAdminEmailConflict
			}
			return nil, domain.ErrAdminUsernameConflict
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	s.logInfo("管理员用户创建成功: " + user.Username)
	return toAdminUserResponse(user), nil
}

// GetUserList 获取管理员用户列表。
func (s *AdminUserService) GetUserList(req *dto.AdminUserListRequest) (*dto.AdminUserListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	status, err := normalizeOptionalAdminStatus(req.Status)
	if err != nil {
		return nil, err
	}
	users, total, err := s.store.GetList(page, pageSize, req.Username, status)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}

	items := make([]*dto.AdminUserResponse, 0, len(users))
	for _, user := range users {
		items = append(items, toAdminUserResponse(user))
	}

	return &dto.AdminUserListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetUserByID 获取管理员用户详情。
func (s *AdminUserService) GetUserByID(id uint) (*dto.AdminUserResponse, error) {
	user, err := s.store.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("查询管理员账号失败: %w", err)
	}
	return toAdminUserResponse(user), nil
}

// UpdateUser 更新管理员用户。
func (s *AdminUserService) UpdateUser(id uint, req *dto.UpdateAdminUserRequest) error {
	user, err := s.store.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrAdminUserNotFound
		}
		return fmt.Errorf("查询管理员账号失败: %w", err)
	}

	usernameChanged := req.Username != "" && req.Username != user.Username
	if usernameChanged {
		if s.store.UsernameExists(req.Username) {
			return domain.ErrAdminUsernameConflict
		}
		user.Username = req.Username
	}
	if req.RealName != "" {
		user.RealName = req.RealName
	}
	if req.Email != nil {
		normalizedEmail, normalizeErr := appHelper.NormalizeAdminEmail(*req.Email)
		if normalizeErr != nil {
			return domain.ErrInvalidAdminEmail
		}
		if normalizedEmail == "" {
			// 历史空邮箱账号可以继续保持空值，但已设置邮箱不允许被清空。
			if user.Email != nil {
				return domain.ErrInvalidAdminEmail
			}
		} else {
			emailExists, existsErr := s.store.EmailExists(normalizedEmail, id)
			if existsErr != nil {
				return fmt.Errorf("检查邮箱失败: %w", existsErr)
			}
			if emailExists {
				return domain.ErrAdminEmailConflict
			}
			user.Email = &normalizedEmail
		}
	}
	if req.Status != nil {
		status, statusErr := normalizeAdminStatus(req.Status, user.Status)
		if statusErr != nil {
			return statusErr
		}
		user.Status = status
	}

	if err := s.store.Update(user); err != nil {
		s.logError("更新用户失败: " + err.Error())
		if isDuplicateKeyError(err) {
			lowerMessage := strings.ToLower(err.Error())
			if strings.Contains(lowerMessage, "username") {
				return domain.ErrAdminUsernameConflict
			}
			if strings.Contains(lowerMessage, "email") {
				return domain.ErrAdminEmailConflict
			}
			if usernameChanged && s.store.UsernameExists(user.Username) {
				return domain.ErrAdminUsernameConflict
			}
			if req.Email != nil && user.Email != nil {
				emailExists, existsErr := s.store.EmailExists(*user.Email, id)
				if existsErr == nil && emailExists {
					return domain.ErrAdminEmailConflict
				}
			}
			if usernameChanged {
				return domain.ErrAdminUsernameConflict
			}
			return domain.ErrAdminEmailConflict
		}
		return fmt.Errorf("更新用户失败: %w", err)
	}

	s.logInfo("管理员用户更新成功: " + user.Username)
	return nil
}

// DeleteUser 删除管理员用户。
func (s *AdminUserService) DeleteUser(id uint) error {
	user, err := s.store.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrAdminUserNotFound
		}
		return fmt.Errorf("查询管理员账号失败: %w", err)
	}

	if err := s.store.Delete(id); err != nil {
		s.logError("删除用户失败: " + err.Error())
		return fmt.Errorf("删除用户失败: %w", err)
	}

	s.logInfo("管理员用户删除成功: " + user.Username)
	return nil
}

// ResetPassword 重置本地管理员用户密码。
func (s *AdminUserService) ResetPassword(id uint, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error) {
	user, err := s.store.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrAdminUserNotFound
		}
		return nil, fmt.Errorf("查询管理员账号失败: %w", err)
	}
	if !isLocalAdminAuthSource(user.AuthSource) {
		return nil, domain.ErrAdminPasswordResetForbidden
	}

	plainPassword := req.Password
	if req.AutoGenerate || plainPassword == "" {
		plainPassword, err = generateRandomPassword(16)
		if err != nil {
			s.logError("生成随机密码失败: " + err.Error())
			return nil, fmt.Errorf("生成随机密码失败")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		s.logError("密码加密失败: " + err.Error())
		return nil, fmt.Errorf("密码加密失败")
	}
	if err := s.store.UpdatePassword(id, string(hashedPassword)); err != nil {
		s.logError("更新密码失败: " + err.Error())
		return nil, fmt.Errorf("更新密码失败: %w", err)
	}

	s.logInfo("管理员用户密码重置成功: " + user.Username)
	return &dto.ResetPasswordResponse{Password: plainPassword}, nil
}

func toAdminUserResponse(user *model.AdminUser) *dto.AdminUserResponse {
	return &dto.AdminUserResponse{
		ID:         user.ID,
		Username:   user.Username,
		RealName:   user.RealName,
		Email:      user.Email,
		AuthSource: normalizedAuthSource(user.AuthSource),
		OIDCBound:  user.OidcSub != nil && strings.TrimSpace(*user.OidcSub) != "",
		Status:     normalizeStoredAdminStatus(user.Status),
		CreatedAt:  user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  user.UpdatedAt.Format(time.RFC3339),
	}
}

func normalizedAuthSource(source string) string {
	if isLocalAdminAuthSource(source) {
		return adminAuthSourceLocal
	}
	// 响应契约只有 local/oidc。未知或损坏的来源按非本地身份暴露，
	// 与敏感操作的 fail-closed 规则保持一致，避免前端错误展示密码重置入口。
	return adminAuthSourceOIDC
}

func isLocalAdminAuthSource(source string) bool {
	return source == "" || source == adminAuthSourceLocal
}

func normalizeAdminStatus(status *int8, defaultValue int8) (int8, error) {
	if status == nil {
		return normalizeStoredAdminStatus(defaultValue), nil
	}
	switch *status {
	case 1:
		return 1, nil
	case 0, 2:
		return 0, nil
	default:
		return 0, domain.ErrInvalidAdminStatus
	}
}

func normalizeOptionalAdminStatus(status *int8) (*int8, error) {
	if status == nil {
		return nil, nil
	}
	normalized, err := normalizeAdminStatus(status, 0)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func normalizeStoredAdminStatus(status int8) int8 {
	if status == 1 {
		return 1
	}
	return 0
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation") ||
		strings.Contains(message, "error 1062") ||
		strings.Contains(message, "sqlstate 23505")
}

func (s *AdminUserService) logInfo(message string) {
	if s.logger != nil {
		s.logger.Info(message)
	}
}

func (s *AdminUserService) logError(message string) {
	if s.logger != nil {
		s.logger.Error(message)
	}
}
