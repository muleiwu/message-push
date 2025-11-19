package dto

// InstallCheckResponse 安装状态检查响应
type InstallCheckResponse struct {
	Installed          bool   `json:"installed"`
	DatabaseConnected  bool   `json:"databaseConnected"`
	DatabaseConfigured bool   `json:"databaseConfigured"`
	Message            string `json:"message,omitempty"`
	CurrentDatabase    string `json:"currentDatabase,omitempty"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `json:"driver,omitempty"`   // mysql, postgresql, sqlite
	Host     string `json:"host"`               // 数据库主机
	Port     int    `json:"port"`               // 数据库端口
	Username string `json:"username"`           // 用户名
	Password string `json:"password"`           // 密码
	Database string `json:"database"`           // 数据库名
	Charset  string `json:"charset,omitempty"`  // 字符集
	FilePath string `json:"filePath,omitempty"` // SQLite 文件路径
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `json:"host"`               // Redis主机
	Port     int    `json:"port"`               // Redis端口
	Password string `json:"password,omitempty"` // 密码
	DB       int    `json:"db"`                 // 数据库索引
}

// AdminAccountInfo 管理员账户信息
type AdminAccountInfo struct {
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
	Email    string `json:"email"`    // 邮箱
	RealName string `json:"realName"` // 真实姓名
	Phone    string `json:"phone"`    // 手机号（可选）
}

// InstallSubmitRequest 安装提交请求
type InstallSubmitRequest struct {
	Database DatabaseConfig   `json:"database"` // 数据库配置
	Redis    RedisConfig      `json:"redis"`    // Redis配置
	Admin    AdminAccountInfo `json:"admin"`    // 管理员账户
}

// InstallSubmitResponse 安装提交响应
type InstallSubmitResponse struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}
