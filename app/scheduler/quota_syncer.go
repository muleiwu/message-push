package scheduler

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// QuotaSyncer 配额同步器
type QuotaSyncer struct {
	logger   gsr.Logger
	redis    *redis.Client
	db       *gorm.DB
	appDao   *dao.ApplicationDAO
	interval time.Duration
	stopCh   chan struct{}
}

// NewQuotaSyncer 创建同步器
func NewQuotaSyncer() *QuotaSyncer {
	h := helper.GetHelper()
	return &QuotaSyncer{
		logger:   h.GetLogger(),
		redis:    h.GetRedis(),
		db:       h.GetDatabase(),
		appDao:   dao.NewApplicationDAO(),
		interval: 1 * time.Hour, // 每小时同步一次
		stopCh:   make(chan struct{}),
	}
}

// Start 启动同步器
func (s *QuotaSyncer) Start(ctx context.Context) error {
	s.logger.Info("quota syncer started")

	go func() {
		// 立即执行一次
		s.sync(ctx)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.sync(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Stop 停止同步器
func (s *QuotaSyncer) Stop() {
	close(s.stopCh)
	s.logger.Info("quota syncer stopped")
}

// sync 同步配额数据
func (s *QuotaSyncer) sync(ctx context.Context) {
	s.logger.Info("starting quota sync...")

	// 1. 获取所有应用
	// 这里假设应用数量不多，可以直接全部获取。如果很多，需要分页。
	var apps []*model.Application
	if err := s.db.Find(&apps).Error; err != nil {
		s.logger.Error(fmt.Sprintf("failed to list applications for sync: %v", err))
		return
	}

	todayStr := time.Now().Format("2006-01-02")
	today, _ := time.Parse("2006-01-02", todayStr)

	for _, app := range apps {
		// 2. 从 Redis 获取今日使用量
		used, _, err := apphelper.GetQuotaUsage(ctx, s.redis, app.ID)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to get quota usage for app %s: %v", app.AppName, err))
			continue
		}

		// 3. 更新或插入 app_quota_stats
		// 使用 Upsert (On Duplicate Key Update)
		stat := model.AppQuotaStat{
			AppID:      app.AppID,
			StatDate:   today,
			TotalCount: int(used),
			// Success/Failed Count 暂时无法从 Redis Quota Key 获取，需要日志聚合
			// 这里先只更新 TotalCount (即配额消耗量)
		}

		// 注意：如果我们只想更新 TotalCount 而保留 Success/Failed (如果已有)，需要小心 Upsert 逻辑
		// GORM Clause
		err = s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "app_id"}, {Name: "stat_date"}},
			DoUpdates: clause.AssignmentColumns([]string{"total_count", "updated_at"}),
		}).Create(&stat).Error

		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to upsert quota stat for app %s: %v", app.AppName, err))
		}
	}

	s.logger.Info("quota sync completed")
}
