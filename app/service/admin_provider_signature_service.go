package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

// AdminProviderSignatureService 签名管理服务
type AdminProviderSignatureService struct {
	signatureDAO *dao.ProviderSignatureDAO
	accountDAO   *dao.ProviderAccountDAO
}

// NewAdminProviderSignatureService 创建签名管理服务实例
func NewAdminProviderSignatureService() *AdminProviderSignatureService {
	db := helper.GetDatabase()
	return &AdminProviderSignatureService{
		signatureDAO: dao.NewProviderSignatureDAO(db),
		accountDAO:   dao.NewProviderAccountDAO(),
	}
}

// GetSignatureList 获取签名列表
func (s *AdminProviderSignatureService) GetSignatureList(providerAccountID uint, status *int8) ([]dto.ProviderSignatureResponse, error) {
	// 验证账号是否存在
	account, err := s.accountDAO.GetByID(providerAccountID)
	if err != nil {
		return nil, fmt.Errorf("provider account not found")
	}
	signatures, err := s.signatureDAO.GetByProviderAccountID(providerAccountID, status)
	if err != nil {
		return nil, err
	}

	// 转换为响应DTO
	responses := make([]dto.ProviderSignatureResponse, 0, len(signatures))
	for _, sig := range signatures {
		item := dto.ProviderSignatureResponse{
			ID:                sig.ID,
			ProviderAccountID: sig.ProviderAccountID,
			SignatureCode:     sig.SignatureCode,
			SignatureName:     sig.SignatureName,
			Status:            sig.Status,
			Remark:            sig.Remark,
			CreatedAt:         sig.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         sig.UpdatedAt.Format(time.RFC3339),
		}
		applySignatureProviderPolicy(&item, account)
		responses = append(responses, item)
	}

	return responses, nil
}

// GetGlobalSignatureList 获取全局签名分页列表。
// 与账号下的非分页列表分开，保持原有嵌套 API 的响应结构不变。
func (s *AdminProviderSignatureService) GetGlobalSignatureList(req *dto.ProviderSignatureListRequest) (*dto.ProviderSignatureListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var providerAccountID *uint
	if req.ProviderAccountID > 0 {
		providerAccountID = &req.ProviderAccountID
	}

	signatures, total, err := s.signatureDAO.List(providerAccountID, req.Status, page, pageSize)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.ProviderSignatureResponse, 0, len(signatures))
	for _, sig := range signatures {
		item := &dto.ProviderSignatureResponse{
			ID:                sig.ID,
			ProviderAccountID: sig.ProviderAccountID,
			SignatureCode:     sig.SignatureCode,
			SignatureName:     sig.SignatureName,
			Status:            sig.Status,
			Remark:            sig.Remark,
			CreatedAt:         sig.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         sig.UpdatedAt.Format(time.RFC3339),
		}
		if sig.ProviderAccount != nil {
			applySignatureProviderPolicy(item, sig.ProviderAccount)
		}
		items = append(items, item)
	}

	return &dto.ProviderSignatureListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// CreateSignature 创建签名
func (s *AdminProviderSignatureService) CreateSignature(providerAccountID uint, req *dto.CreateProviderSignatureRequest) (*dto.ProviderSignatureResponse, error) {
	// 验证账号是否存在
	account, err := s.accountDAO.GetByID(providerAccountID)
	if err != nil {
		return nil, fmt.Errorf("provider account not found")
	}
	if err := ensureSignatureWritable(account); err != nil {
		return nil, err
	}

	// 检查签名代码是否已存在
	exists, err := s.signatureDAO.CheckSignatureExists(providerAccountID, req.SignatureCode, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("signature code already exists for this account")
	}

	// 创建签名
	signature := &model.ProviderSignature{
		ProviderAccountID: providerAccountID,
		SignatureCode:     req.SignatureCode,
		SignatureName:     req.SignatureName,
		Status:            req.Status,
		Remark:            req.Remark,
	}

	if err := s.signatureDAO.Create(signature); err != nil {
		return nil, err
	}

	response := &dto.ProviderSignatureResponse{
		ID:                signature.ID,
		ProviderAccountID: signature.ProviderAccountID,
		SignatureCode:     signature.SignatureCode,
		SignatureName:     signature.SignatureName,
		Status:            signature.Status,
		Remark:            signature.Remark,
		CreatedAt:         signature.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         signature.UpdatedAt.Format(time.RFC3339),
	}
	applySignatureProviderPolicy(response, account)
	return response, nil
}

// UpdateSignature 更新签名
func (s *AdminProviderSignatureService) UpdateSignature(id uint, req *dto.UpdateProviderSignatureRequest) error {
	// 获取原签名
	signature, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("signature not found")
	}
	if err := ensureSignatureWritable(signature.ProviderAccount); err != nil {
		return err
	}

	// 检查签名代码是否与其他签名冲突
	exists, err := s.signatureDAO.CheckSignatureExists(signature.ProviderAccountID, req.SignatureCode, &id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("signature code already exists for this account")
	}

	// 更新字段
	signature.SignatureCode = req.SignatureCode
	signature.SignatureName = req.SignatureName
	signature.Status = req.Status
	signature.Remark = req.Remark

	return s.signatureDAO.Update(signature)
}

// DeleteSignature 删除签名
func (s *AdminProviderSignatureService) DeleteSignature(id uint) error {
	// 获取签名
	signature, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("signature not found")
	}
	if err := ensureSignatureWritable(signature.ProviderAccount); err != nil {
		return err
	}

	return s.signatureDAO.Delete(id)
}

// GetSignatureByID 根据ID获取签名
func (s *AdminProviderSignatureService) GetSignatureByID(id uint) (*dto.ProviderSignatureResponse, error) {
	signature, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("signature not found")
	}

	response := &dto.ProviderSignatureResponse{
		ID:                signature.ID,
		ProviderAccountID: signature.ProviderAccountID,
		SignatureCode:     signature.SignatureCode,
		SignatureName:     signature.SignatureName,
		Status:            signature.Status,
		Remark:            signature.Remark,
		CreatedAt:         signature.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         signature.UpdatedAt.Format(time.RFC3339),
	}
	applySignatureProviderPolicy(response, signature.ProviderAccount)
	return response, nil
}

func applySignatureProviderPolicy(response *dto.ProviderSignatureResponse, account *model.ProviderAccount) {
	if response == nil || account == nil {
		return
	}
	response.ProviderAccountID = account.ID
	response.ProviderAccountName = account.AccountName
	response.ProviderCode = account.ProviderCode
	response.ProviderType = account.ProviderType
	if meta, err := registry.GetByCode(account.ProviderCode); err == nil {
		response.RequiresSignature = meta.RequiresSignature
	}
	response.HistoricalOnly = account.ProviderType == constants.MessageTypeEmail && !response.RequiresSignature
	response.ReadOnly = !response.RequiresSignature
}

func ensureSignatureWritable(account *model.ProviderAccount) error {
	if account == nil {
		return fmt.Errorf("provider account not found")
	}
	meta, err := registry.GetByCode(account.ProviderCode)
	if err != nil {
		return fmt.Errorf("provider is not registered: %w", err)
	}
	if !meta.RequiresSignature {
		return fmt.Errorf("provider does not require signatures; historical signatures are read-only")
	}
	return nil
}
