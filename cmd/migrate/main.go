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

	// 使用统一的模型列表
	models := autoload.Migration{}.Get()

	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			log.Fatalf("Failed to migrate %T: %v", m, err)
		}
	}

	log.Println("Migrations completed successfully!")
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
		"push_channels",
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
