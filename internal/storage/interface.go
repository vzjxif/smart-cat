package storage

import (
	"time"
	"smart-cat/internal/smart"
)

// Storage 存储接口
type Storage interface {
	// SaveRecord 保存一条 SMART 数据记录
	SaveRecord(serial string, data *smart.SMARTData) error

	// GetHistory 获取指定设备的历史记录
	GetHistory(serial string, from, to time.Time) ([]smart.HistoryRecord, error)

	// GetAllSerials 获取所有已记录的设备序列号
	GetAllSerials() ([]string, error)

	// CleanOldRecords 清理旧记录
	CleanOldRecords(days int) error
}