package constants

// 任务状态常量
const (
	TaskStatusPending    = "pending"    // 待处理
	TaskStatusProcessing = "processing" // 处理中
	TaskStatusSuccess    = "success"    // 成功
	TaskStatusFailed     = "failed"     // 失败
)

// 批量任务状态
const (
	BatchStatusProcessing = "processing" // 处理中
	BatchStatusCompleted  = "completed"  // 已完成
	BatchStatusFailed     = "failed"     // 失败
)

// IsValidTaskStatus 检查任务状态是否有效
func IsValidTaskStatus(status string) bool {
	switch status {
	case TaskStatusPending, TaskStatusProcessing, TaskStatusSuccess, TaskStatusFailed:
		return true
	default:
		return false
	}
}

// IsValidBatchStatus 检查批量任务状态是否有效
func IsValidBatchStatus(status string) bool {
	switch status {
	case BatchStatusProcessing, BatchStatusCompleted, BatchStatusFailed:
		return true
	default:
		return false
	}
}
