package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/config"
	"cnb.cool/mliev/push/message-push/config/autoload"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	action := flag.String("action", "", "migrate action: up, down, fresh, seed")
	flag.Parse()

	if *action == "" {
		fmt.Println("Usage: go run cmd/migrate/main.go -action=<up|down|fresh|seed>")
		os.Exit(1)
	}

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

	// 执行迁移
	switch *action {
	case "up":
		migrateUp(db)
	case "down":
		migrateDown(db)
	case "fresh":
		migrateFresh(db)
	case "seed":
		seed(db)
	default:
		log.Fatal("Invalid action:", *action)
	}
}

// migrateUp 执行迁移
func migrateUp(db *gorm.DB) {
	log.Println("Running migrations...")

	// 在 AutoMigrate 之前执行预迁移清理
	if err := preMigrationCleanup(db); err != nil {
		log.Fatalf("Failed to run pre-migration cleanup: %v", err)
	}

	// 使用统一的模型列表
	models := autoload.Migration{}.Get()

	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			log.Fatalf("Failed to migrate %T: %v", m, err)
		}
	}

	// 执行自定义迁移
	if err := customMigrations(db); err != nil {
		log.Fatalf("Failed to run custom migrations: %v", err)
	}

	log.Println("Migrations completed successfully!")
}

// preMigrationCleanup 在 AutoMigrate 之前执行清理工作
func preMigrationCleanup(db *gorm.DB) error {
	log.Println("Running pre-migration cleanup...")

	// 修复 channel_template_bindings 表的旧字段和无效数据
	if err := fixChannelTemplateBindingsPreMigration(db); err != nil {
		return fmt.Errorf("failed to fix channel_template_bindings: %w", err)
	}

	log.Println("Pre-migration cleanup completed!")
	return nil
}

// customMigrations 执行自定义迁移逻辑
func customMigrations(db *gorm.DB) error {
	log.Println("Running custom migrations...")

	log.Println("Custom migrations completed!")
	return nil
}

// fixChannelTemplateBindingsPreMigration 在 AutoMigrate 前修复 channel_template_bindings 表
func fixChannelTemplateBindingsPreMigration(db *gorm.DB) error {
	// 检查表是否存在
	if !db.Migrator().HasTable("channel_template_bindings") {
		log.Println("Table channel_template_bindings does not exist, skipping...")
		return nil
	}

	log.Println("Fixing channel_template_bindings table...")

	// 步骤 1: 删除旧的 template_binding_id 外键约束（如果存在）
	if db.Migrator().HasColumn(&model.ChannelTemplateBinding{}, "template_binding_id") {
		log.Println("Found template_binding_id column, removing foreign key constraint...")

		var constraintName string
		err := db.Raw(`
			SELECT CONSTRAINT_NAME 
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE 
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = 'channel_template_bindings'
			  AND COLUMN_NAME = 'template_binding_id'
			  AND REFERENCED_TABLE_NAME IS NOT NULL
			LIMIT 1
		`).Scan(&constraintName).Error

		if err == nil && constraintName != "" {
			log.Printf("Dropping foreign key constraint: %s", constraintName)
			if err := db.Exec(fmt.Sprintf("ALTER TABLE channel_template_bindings DROP FOREIGN KEY %s", constraintName)).Error; err != nil {
				return fmt.Errorf("failed to drop foreign key: %w", err)
			}
		}

		// 删除旧字段
		log.Println("Dropping template_binding_id column...")
		if err := db.Migrator().DropColumn(&model.ChannelTemplateBinding{}, "template_binding_id"); err != nil {
			return fmt.Errorf("failed to drop column: %w", err)
		}
	}

	// 步骤 2: 临时删除可能存在的外键约束，以便清理无效数据
	log.Println("Temporarily dropping foreign key constraints...")
	foreignKeys := []string{
		"fk_channel_template_bindings_channel",
		"fk_channel_template_bindings_provider_template",
		"fk_channel_template_bindings_provider_account",
	}

	for _, fk := range foreignKeys {
		// 检查外键是否存在
		var count int64
		db.Raw(`
			SELECT COUNT(*) 
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS 
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = 'channel_template_bindings'
			  AND CONSTRAINT_NAME = ?
			  AND CONSTRAINT_TYPE = 'FOREIGN KEY'
		`, fk).Scan(&count)

		if count > 0 {
			log.Printf("Dropping foreign key: %s", fk)
			if err := db.Exec(fmt.Sprintf("ALTER TABLE channel_template_bindings DROP FOREIGN KEY %s", fk)).Error; err != nil {
				log.Printf("Warning: Failed to drop foreign key %s: %v", fk, err)
			}
		}
	}

	// 步骤 3: 清理无效数据
	log.Println("Cleaning invalid data...")

	// 删除引用不存在的 provider_template_id 的记录
	result := db.Exec(`
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN provider_templates pt ON ctb.provider_template_id = pt.id
		WHERE ctb.provider_template_id IS NOT NULL 
		  AND pt.id IS NULL
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to clean invalid provider_template_id references: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		log.Printf("Deleted %d records with invalid provider_template_id", result.RowsAffected)
	}

	// 删除引用不存在的 provider_id 的记录
	result = db.Exec(`
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN provider_accounts pa ON ctb.provider_id = pa.id
		WHERE ctb.provider_id IS NOT NULL 
		  AND pa.id IS NULL
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to clean invalid provider_id references: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		log.Printf("Deleted %d records with invalid provider_id", result.RowsAffected)
	}

	// 删除引用不存在的 channel_id 的记录
	result = db.Exec(`
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN channels c ON ctb.channel_id = c.id
		WHERE ctb.channel_id IS NOT NULL 
		  AND c.id IS NULL
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to clean invalid channel_id references: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		log.Printf("Deleted %d records with invalid channel_id", result.RowsAffected)
	}

	log.Println("Successfully fixed channel_template_bindings table")
	return nil
}

// migrateDown 回滚迁移
func migrateDown(db *gorm.DB) {
	log.Println("Rolling back migrations...")

	// 删除所有表（按依赖关系倒序）
	tables := []string{
		"admin_users",
		"provider_quota_stats",
		"app_quota_stats",
		"channel_health_history",
		"push_logs",
		"push_batch_tasks",
		"push_tasks",
		"channel_provider_relations",
		"provider_channels",
		"channels",
		"provider_accounts", // 新表
		"providers",         // 旧表
		"applications",
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			log.Printf("Failed to drop table %s: %v", table, err)
		}
	}

	log.Println("Rollback completed successfully!")
}

// migrateFresh 清空并重新迁移
func migrateFresh(db *gorm.DB) {
	log.Println("Fresh migration...")
	migrateDown(db)
	migrateUp(db)
}

// seed 填充测试数据
func seed(db *gorm.DB) {
	log.Println("Seeding data...")

	// 创建默认管理员账号
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
	} else {
		adminUser := &model.AdminUser{
			Username: "admin",
			Password: string(hashedPassword),
			RealName: "系统管理员",
			Status:   1,
		}
		if err := db.Create(adminUser).Error; err != nil {
			log.Printf("Failed to create admin user: %v", err)
		} else {
			log.Println("Default admin user created (username: admin, password: admin123)")
		}
	}

	// 创建测试应用
	app := &model.Application{
		AppID:      "test_app_001",
		AppSecret:  "test_secret_please_change_in_production",
		AppName:    "测试应用",
		Status:     1,
		DailyQuota: 10000,
		RateLimit:  100,
	}
	db.Create(app)

	// 创建服务商账号配置
	providerAccount := &model.ProviderAccount{
		AccountCode:  "aliyun_sms_001",
		AccountName:  "阿里云短信测试账号",
		ProviderCode: "aliyun_sms",
		ProviderType: "sms",
		Config:       `{"access_key_id":"your_key","access_key_secret":"your_secret","sign_name":"测试签名"}`,
		Status:       1,
		Remark:       "测试服务商账号，请在生产环境中修改配置",
	}
	db.Create(providerAccount)

	log.Println("Seeding completed!")
}
