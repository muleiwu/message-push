package migration

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

type Migration struct {
	Helper    interfaces.HelperInterface
	Migration []any
}

func (receiver *Migration) Run() error {

	if !receiver.Helper.GetConfig().GetBool("app.installed", false) {
		receiver.Helper.GetLogger().Warn("[db migration] 数据库未安装，不执行迁移")
		return nil
	}
	if len(receiver.Migration) > 0 {
		err := receiver.Helper.GetDatabase().AutoMigrate(receiver.Migration...)
		if err != nil {
			return fmt.Errorf("[db migration err:%s]", err.Error())
		}

		receiver.Helper.GetLogger().Info(fmt.Sprintf("[db migration success: %d models migrated]", len(receiver.Migration)))
	}
	return nil
}

// Stop Migration 服务不需要停止操作，空实现
func (receiver *Migration) Stop() error {
	return nil
}
