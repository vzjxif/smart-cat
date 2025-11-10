package service

import (
	"time"

	"smart-cat/internal/smart"
	"smart-cat/internal/storage"
)

// DeviceService 设备管理服务
type DeviceService struct {
	detector *smart.DeviceDetector
	storage  storage.Storage
}

// NewDeviceService 创建设备服务
func NewDeviceService(detector *smart.DeviceDetector, storage storage.Storage) *DeviceService {
	return &DeviceService{
		detector: detector,
		storage:  storage,
	}
}

// GetAllDevices 获取所有设备信息
func (s *DeviceService) GetAllDevices() ([]smart.DeviceInfo, error) {
	devices, err := s.detector.ListDevices()
	if err != nil {
		return nil, err
	}

	// 获取所有已记录的序列号
	serials, _ := s.storage.GetAllSerials()
	serialMap := make(map[string]bool)
	for _, s := range serials {
		serialMap[s] = true
	}

	var deviceInfos []smart.DeviceInfo
	for _, device := range devices {
		// 尝试获取 SMART 数据来检测设备是否可读
		data, err := s.detector.GetSMARTData(device.Name)
		if err != nil {
			// 添加无法读取的设备信息
			deviceInfos = append(deviceInfos, smart.DeviceInfo{
				Device: smart.Device{
					Name:       device.Name,
					Model:      "无法读取",
					Serial:     "unknown",
					DeviceType: "Unknown",
					CapacityGB: device.CapacityGB,
					IsExternal: device.IsExternal,
				},
				HasHistory:   false,
				Error:        err.Error(),
				ErrorMessage: "无法读取 SMART 数据（可能是不支持的 USB 桥接芯片）",
			})
			continue
		}

		deviceInfos = append(deviceInfos, smart.DeviceInfo{
			Device:     data.Device,
			HasHistory: serialMap[data.Device.Serial],
		})
	}

	return deviceInfos, nil
}

// GetSMARTData 获取指定设备的实时 SMART 数据
func (s *DeviceService) GetSMARTData(deviceName string) (*smart.SMARTData, error) {
	return s.detector.GetSMARTData(deviceName)
}

// GetHistory 获取指定设备的历史数据
func (s *DeviceService) GetHistory(serial string, from, to time.Time) ([]smart.HistoryRecord, error) {
	// 默认最近7天
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -7)
	}

	return s.storage.GetHistory(serial, from, to)
}

// CleanOldRecords 清理旧记录
func (s *DeviceService) CleanOldRecords(days int) error {
	return s.storage.CleanOldRecords(days)
}

// CheckDependencies 检查系统依赖
func (s *DeviceService) CheckDependencies() error {
	return s.detector.CheckSmartctlInstalled()
}