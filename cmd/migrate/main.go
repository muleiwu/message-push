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

	// 移除 message_templates 表的 template_code 字段
	if err := removeMessageTemplateCodeColumn(db); err != nil {
		return fmt.Errorf("failed to remove template_code from message_templates: %w", err)
	}

	// 将 provider_msg_id 从 push_tasks 迁移到 push_logs
	if err := migrateProviderMsgIDToPushLogs(db); err != nil {
		return fmt.Errorf("failed to migrate provider_msg_id to push_logs: %w", err)
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

// removeMessageTemplateCodeColumn 移除 message_templates 表的 template_code 字段
func removeMessageTemplateCodeColumn(db *gorm.DB) error {
	// 检查表是否存在
	if !db.Migrator().HasTable("message_templates") {
		log.Println("Table message_templates does not exist, skipping...")
		return nil
	}

	// 检查 template_code 列是否存在
	if !db.Migrator().HasColumn(&model.MessageTemplate{}, "template_code") {
		log.Println("Column template_code does not exist in message_templates, skipping...")
		return nil
	}

	log.Println("Removing template_code column from message_templates table...")

	// 步骤 1: 删除唯一索引 uk_template_code（如果存在）
	var indexExists int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'message_templates'
		  AND INDEX_NAME = 'uk_template_code'
	`).Scan(&indexExists)

	if indexExists > 0 {
		log.Println("Dropping unique index uk_template_code...")
		if err := db.Exec("ALTER TABLE message_templates DROP INDEX uk_template_code").Error; err != nil {
			log.Printf("Warning: Failed to drop index uk_template_code: %v", err)
		}
	}

	// 步骤 2: 删除 template_code 列
	log.Println("Dropping template_code column...")
	if err := db.Migrator().DropColumn(&model.MessageTemplate{}, "template_code"); err != nil {
		return fmt.Errorf("failed to drop template_code column: %w", err)
	}

	log.Println("Successfully removed template_code column from message_templates table")
	return nil
}

// migrateProviderMsgIDToPushLogs 将 provider_msg_id 从 push_tasks 迁移到 push_logs
func migrateProviderMsgIDToPushLogs(db *gorm.DB) error {
	// 检查 push_tasks 表是否存在
	if !db.Migrator().HasTable("push_tasks") {
		log.Println("Table push_tasks does not exist, skipping provider_msg_id migration...")
		return nil
	}

	// 检查 push_tasks 表是否有 provider_msg_id 列
	if !db.Migrator().HasColumn(&model.PushTask{}, "provider_msg_id") {
		log.Println("Column provider_msg_id does not exist in push_tasks, skipping migration...")
		return nil
	}

	log.Println("Migrating provider_msg_id from push_tasks to push_logs...")

	// 检查 push_logs 表是否存在
	if !db.Migrator().HasTable("push_logs") {
		log.Println("Table push_logs does not exist, will be created by AutoMigrate...")
		// 先创建 push_logs 表（如果不存在），以便添加 provider_msg_id 列
		if err := db.AutoMigrate(&model.PushLog{}); err != nil {
			return fmt.Errorf("failed to create push_logs table: %w", err)
		}
	}

	// 检查 push_logs 表是否已有 provider_msg_id 列
	if !db.Migrator().HasColumn(&model.PushLog{}, "provider_msg_id") {
		log.Println("Adding provider_msg_id column to push_logs...")
		if err := db.Exec("ALTER TABLE push_logs ADD COLUMN provider_msg_id VARCHAR(100) DEFAULT NULL COMMENT '服务商返回的消息ID'").Error; err != nil {
			return fmt.Errorf("failed to add provider_msg_id column to push_logs: %w", err)
		}

		// 添加索引
		log.Println("Adding index idx_provider_msg_id on push_logs...")
		if err := db.Exec("ALTER TABLE push_logs ADD INDEX idx_provider_msg_id (provider_msg_id)").Error; err != nil {
			log.Printf("Warning: Failed to add index idx_provider_msg_id: %v", err)
		}
	}

	// 迁移数据：将 push_tasks 中的 provider_msg_id 更新到对应的 push_logs 记录
	// 策略：对于每个有 provider_msg_id 的 task，更新其最新的 push_log 记录
	log.Println("Migrating provider_msg_id data...")
	result := db.Exec(`
		UPDATE push_logs pl
		INNER JOIN (
			SELECT task_id, provider_msg_id 
			FROM push_tasks 
			WHERE provider_msg_id IS NOT NULL AND provider_msg_id != ''
		) pt ON pl.task_id = pt.task_id
		INNER JOIN (
			SELECT task_id, MAX(id) as max_id
			FROM push_logs
			GROUP BY task_id
		) latest ON pl.task_id = latest.task_id AND pl.id = latest.max_id
		SET pl.provider_msg_id = pt.provider_msg_id
		WHERE pl.provider_msg_id IS NULL OR pl.provider_msg_id = ''
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to migrate provider_msg_id data: %w", result.Error)
	}
	log.Printf("Migrated %d push_log records with provider_msg_id", result.RowsAffected)

	// 删除 push_tasks 表的 provider_msg_id 索引（如果存在）
	var indexExists int64
	db.Raw(`
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'push_tasks'
		  AND INDEX_NAME = 'idx_provider_msg_id'
	`).Scan(&indexExists)

	if indexExists > 0 {
		log.Println("Dropping index idx_provider_msg_id from push_tasks...")
		if err := db.Exec("ALTER TABLE push_tasks DROP INDEX idx_provider_msg_id").Error; err != nil {
			log.Printf("Warning: Failed to drop index idx_provider_msg_id: %v", err)
		}
	}

	// 删除 push_tasks 表的 provider_msg_id 列
	log.Println("Dropping provider_msg_id column from push_tasks...")
	if err := db.Exec("ALTER TABLE push_tasks DROP COLUMN provider_msg_id").Error; err != nil {
		return fmt.Errorf("failed to drop provider_msg_id column from push_tasks: %w", err)
	}

	log.Println("Successfully migrated provider_msg_id from push_tasks to push_logs")
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

	// 创建默认失败规则
	seedFailureRules(db)

	log.Println("Seeding completed!")
}

// seedFailureRules 初始化默认失败规则
func seedFailureRules(db *gorm.DB) {
	log.Println("Seeding failure rules...")

	rules := []*model.FailureRule{
		// 发送失败场景规则
		{
			Name:         "余额不足告警",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "余额不足,insufficient balance",
			Action:       model.RuleActionAlert,
			ActionConfig: `{"alert_level":"critical"}`,
			Priority:     100,
			Status:       1,
			Remark:       "余额不足时发送告警通知，不重试",
		},
		{
			Name:         "黑名单不重试",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "黑名单,blacklist",
			Action:       model.RuleActionFail,
			Priority:     90,
			Status:       1,
			Remark:       "黑名单号码直接失败，不重试",
		},
		{
			Name:         "签名未审核不重试",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "签名未审核,signature not approved",
			Action:       model.RuleActionFail,
			Priority:     90,
			Status:       1,
			Remark:       "签名问题需要人工处理，不重试",
		},
		{
			Name:         "模板未审核不重试",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "模板未审核,template not approved",
			Action:       model.RuleActionFail,
			Priority:     90,
			Status:       1,
			Remark:       "模板问题需要人工处理，不重试",
		},
		{
			Name:         "手机号无效不重试",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "手机号无效,invalid phone,invalid mobile",
			Action:       model.RuleActionFail,
			Priority:     80,
			Status:       1,
			Remark:       "手机号格式错误，不重试",
		},
		{
			Name:         "参数错误不重试",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "参数错误,invalid parameter",
			Action:       model.RuleActionFail,
			Priority:     80,
			Status:       1,
			Remark:       "参数错误，不重试",
		},
		{
			Name:         "网络超时切换供应商",
			Scene:        model.RuleSceneSendFailure,
			ErrorKeyword: "timeout,超时,network error",
			Action:       model.RuleActionSwitchProvider,
			ActionConfig: `{"exclude_current":true,"max_retry":2}`,
			Priority:     70,
			Status:       1,
			Remark:       "网络超时时尝试切换供应商",
		},
		{
			Name:         "发送失败默认重试",
			Scene:        model.RuleSceneSendFailure,
			Action:       model.RuleActionRetry,
			ActionConfig: `{"max_retry":3,"delay_seconds":2,"backoff_rate":2,"max_delay":60}`,
			Priority:     0,
			Status:       1,
			Remark:       "默认规则：其他发送失败情况重试3次",
		},
		// 回调失败场景规则
		{
			Name:         "用户关机切换供应商",
			Scene:        model.RuleSceneCallbackFailure,
			ErrorKeyword: "关机,power off,shutdown",
			Action:       model.RuleActionSwitchProvider,
			ActionConfig: `{"exclude_current":true,"max_retry":1}`,
			Priority:     50,
			Status:       1,
			Remark:       "用户关机时尝试切换供应商重发",
		},
		{
			Name:         "号码停机不重试",
			Scene:        model.RuleSceneCallbackFailure,
			ErrorKeyword: "停机,suspended,out of service",
			Action:       model.RuleActionFail,
			Priority:     80,
			Status:       1,
			Remark:       "号码停机直接失败",
		},
		{
			Name:     "回调失败默认不重试",
			Scene:    model.RuleSceneCallbackFailure,
			Action:   model.RuleActionFail,
			Priority: 0,
			Status:   1,
			Remark:   "默认规则：回调失败不重试",
		},
	}

	for _, rule := range rules {
		// 检查是否已存在同名规则
		var count int64
		db.Model(&model.FailureRule{}).Where("name = ?", rule.Name).Count(&count)
		if count > 0 {
			log.Printf("Rule already exists: %s, skipping...", rule.Name)
			continue
		}

		if err := db.Create(rule).Error; err != nil {
			log.Printf("Failed to create rule %s: %v", rule.Name, err)
		} else {
			log.Printf("Created rule: %s", rule.Name)
		}
	}

	log.Println("Failure rules seeding completed!")
}
