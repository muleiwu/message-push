package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminProviderSignatureService 签名管理服务
type AdminProviderSignatureService struct {
	signatureDAO *dao.ProviderSignatureDAO
	accountDAO   *dao.ProviderAccountDAO
}

// NewAdminProviderSignatureService 创建签名管理服务实例
func NewAdminProviderSignatureService() *AdminProviderSignatureService {
	db := helper.GetHelper().GetDatabase()
	return &AdminProviderSignatureService{
		signatureDAO: dao.NewProviderSignatureDAO(db),
		accountDAO:   dao.NewProviderAccountDAO(),
	}
}

// GetSignatureList 获取签名列表
func (s *AdminProviderSignatureService) GetSignatureList(providerAccountID uint, status *int8) ([]dto.ProviderSignatureResponse, error) {
	// 验证账号是否存在
	_, err := s.accountDAO.GetByID(providerAccountID)
	if err != nil {
		return nil, fmt.Errorf("provider account not found")
	}

	signatures, err := s.signatureDAO.GetByProviderAccountID(providerAccountID, status)
	if err != nil {
		return nil, err
	}

	// 转换为响应DTO
	var responses []dto.ProviderSignatureResponse
	for _, sig := range signatures {
		responses = append(responses, dto.ProviderSignatureResponse{
			ID:                sig.ID,
			ProviderAccountID: sig.ProviderAccountID,
			SignatureCode:     sig.SignatureCode,
			SignatureName:     sig.SignatureName,
			Status:            sig.Status,
			Remark:            sig.Remark,
			CreatedAt:         sig.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         sig.UpdatedAt.Format(time.RFC3339),
		})
	}

	return responses, nil
}

// CreateSignature 创建签名
func (s *AdminProviderSignatureService) CreateSignature(providerAccountID uint, req *dto.CreateProviderSignatureRequest) (*dto.ProviderSignatureResponse, error) {
	// 验证账号是否存在
	account, err := s.accountDAO.GetByID(providerAccountID)
	if err != nil {
		return nil, fmt.Errorf("provider account not found")
	}

	// 验证账号类型是否为SMS
	if account.ProviderType != "sms" {
		return nil, fmt.Errorf("only SMS provider accounts can have signatures")
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

	return &dto.ProviderSignatureResponse{
		ID:                signature.ID,
		ProviderAccountID: signature.ProviderAccountID,
		SignatureCode:     signature.SignatureCode,
		SignatureName:     signature.SignatureName,
		Status:            signature.Status,
		Remark:            signature.Remark,
		CreatedAt:         signature.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         signature.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateSignature 更新签名
func (s *AdminProviderSignatureService) UpdateSignature(id uint, req *dto.UpdateProviderSignatureRequest) error {
	// 获取原签名
	signature, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("signature not found")
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
	_, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return fmt.Errorf("signature not found")
	}

	return s.signatureDAO.Delete(id)
}

// GetSignatureByID 根据ID获取签名
func (s *AdminProviderSignatureService) GetSignatureByID(id uint) (*dto.ProviderSignatureResponse, error) {
	signature, err := s.signatureDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("signature not found")
	}

	return &dto.ProviderSignatureResponse{
		ID:                signature.ID,
		ProviderAccountID: signature.ProviderAccountID,
		SignatureCode:     signature.SignatureCode,
		SignatureName:     signature.SignatureName,
		Status:            signature.Status,
		Remark:            signature.Remark,
		CreatedAt:         signature.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         signature.UpdatedAt.Format(time.RFC3339),
	}, nil
}
