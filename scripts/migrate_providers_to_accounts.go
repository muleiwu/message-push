package main

import (
	"log"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/config"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// 这个脚本用于将旧的providers表数据迁移到新的provider_accounts表
func main() {
	log.Println("开始迁移Provider数据到ProviderAccount...")

	// 初始化配置和数据库
	h := helper.GetHelper()
	assembly := &config.Assembly{Helper: h}
	for _, a := range assembly.Get() {
		if err := a.Assembly(); err != nil {
			log.Fatal("Failed to initialize:", err)
		}
	}

	db := h.GetDatabase()
	if db == nil {
		log.Fatal("Database not initialized")
	}

	// 检查providers表是否存在
	if !db.Migrator().HasTable("providers") {
		log.Println("providers表不存在，无需迁移")
		return
	}

	// 检查provider_accounts表是否存在
	if !db.Migrator().HasTable("provider_accounts") {
		log.Println("provider_accounts表不存在，先创建表...")
		if err := db.AutoMigrate(&model.ProviderAccount{}); err != nil {
			log.Fatal("创建provider_accounts表失败:", err)
		}
	}

	// 查询所有providers
	var providers []model.Provider
	if err := db.Find(&providers).Error; err != nil {
		log.Fatal("查询providers失败:", err)
	}

	log.Printf("找到 %d 条Provider记录", len(providers))

	// 迁移数据
	successCount := 0
	for _, provider := range providers {
		// 创建对应的ProviderAccount
		account := model.ProviderAccount{
			ID:           provider.ID, // 保持ID一致，以维持外键关系
			AccountCode:  provider.ProviderCode,
			AccountName:  provider.ProviderName,
			ProviderCode: provider.ProviderCode, // 原来的ProviderCode作为服务商代码
			ProviderType: provider.ProviderType,
			Config:       provider.Config,
			Status:       provider.Status,
			Remark:       provider.Remark,
			CreatedAt:    provider.CreatedAt,
			UpdatedAt:    provider.UpdatedAt,
			DeletedAt:    provider.DeletedAt,
		}

		// 使用Create而不是Save，避免冲突
		// 如果记录已存在（ID已存在），则跳过
		var existingAccount model.ProviderAccount
		err := db.First(&existingAccount, provider.ID).Error
		if err == nil {
			log.Printf("ProviderAccount ID=%d 已存在，跳过", provider.ID)
			continue
		}

		// 插入新记录，明确指定ID
		if err := db.Create(&account).Error; err != nil {
			log.Printf("迁移Provider ID=%d 失败: %v", provider.ID, err)
			continue
		}

		successCount++
		log.Printf("已迁移: ID=%d, Code=%s, Name=%s", provider.ID, provider.ProviderCode, provider.ProviderName)
	}

	log.Printf("迁移完成！成功迁移 %d 条记录", successCount)
	log.Println("\n注意事项：")
	log.Println("1. provider_channels表中的provider_id外键仍然指向相同的ID")
	log.Println("2. 旧的providers表已保留，如需删除请手动执行")
	log.Println("3. 建议在删除旧表前备份数据")
}
