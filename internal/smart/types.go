package smart

import "time"

// Device 表示一个存储设备
type Device struct {
	Name       string `json:"name"`        // 设备名称 /dev/sda
	Model      string `json:"model"`       // 型号
	Serial     string `json:"serial"`      // 序列号
	DeviceType string `json:"device_type"` // HDD/SSD/NVMe
	CapacityGB int64  `json:"capacity_gb"` // 容量(GB)
	IsExternal bool   `json:"is_external"` // 是否为外置设备
}

// DeviceInfo 设备信息（用于API响应）
type DeviceInfo struct {
	Device
	HasHistory   bool   `json:"has_history"`
	Error        string `json:"error,omitempty"`        // 错误信息
	ErrorMessage string `json:"error_message,omitempty"` // 用户友好的错误消息
}

// SMARTData 表示 SMART 数据快照
type SMARTData struct {
	Device              Device         `json:"device"`
	Temperature         int            `json:"temperature"`          // 温度 °C
	PowerOnHours        int64          `json:"power_on_hours"`       // 通电时间
	PowerCycleCount     int64          `json:"power_cycle_count"`    // 通电次数
	ReallocatedSectors  int64          `json:"reallocated_sectors"`  // 重映射扇区
	PendingSectors      int64          `json:"pending_sectors"`      // 待映射扇区
	UncorrectableErrors int64          `json:"uncorrectable_errors"` // 不可纠正错误
	HealthPercent       int            `json:"health_percent"`       // 健康度百分比
	SmartStatus         string         `json:"smart_status"`         // PASSED/FAILED
	Attributes          []SMARTAttribute `json:"attributes"`           // 所有属性
	Timestamp           time.Time      `json:"timestamp"`            // 数据采集时间
}

// SMARTAttribute 表示单个 SMART 属性
type SMARTAttribute struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Value      int    `json:"value"`
	Worst      int    `json:"worst"`
	Threshold  int    `json:"threshold"`
	RawValue   int64  `json:"raw_value"`
	WhenFailed string `json:"when_failed,omitempty"`
}

// HistoryRecord 表示历史记录中的一条数据
type HistoryRecord struct {
	Timestamp           time.Time `json:"timestamp"`
	Temperature         int       `json:"temperature"`
	PowerOnHours        int64     `json:"power_on_hours"`
	PowerCycleCount     int64     `json:"power_cycle_count"`
	ReallocatedSectors  int64     `json:"reallocated_sectors"`
	PendingSectors      int64     `json:"pending_sectors"`
	UncorrectableErrors int64     `json:"uncorrectable_errors"`
	HealthPercent       int       `json:"health_percent"`
}

// CollectorConfig 采集器配置
type CollectorConfig struct {
	Interval    time.Duration // 采集间隔
	DataDir     string        // 数据存储目录
	Enabled     bool          // 是否启用采集
}

// DefaultCollectorConfig 默认采集器配置
func DefaultCollectorConfig() *CollectorConfig {
	return &CollectorConfig{
		Interval: time.Hour,
		DataDir:  "./data",
		Enabled:  true,
	}
}