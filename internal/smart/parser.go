package smart

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// smartctlOutput smartctl JSON 输出结构（只解析需要的字段）
type smartctlOutput struct {
	Smartctl struct {
		Messages []struct {
			String   string `json:"string"`
			Severity string `json:"severity"`
		} `json:"messages"`
	} `json:"smartctl"`
	Device struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"device"`
	ModelName       string `json:"model_name"`
	SerialNumber    string `json:"serial_number"`
	RotationRate    int    `json:"rotation_rate"`    // 0 = SSD, >0 = HDD RPM
	Trim            struct {
		Supported bool `json:"supported"` // TRIM 支持表示 SSD
	} `json:"trim"`
	SmartStatus     struct {
		Passed bool `json:"passed"`
	} `json:"smart_status"`
	Temperature struct {
		Current int `json:"current"`
	} `json:"temperature"`
	PowerOnTime struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`
	PowerCycleCount int64 `json:"power_cycle_count"`
	AtaSmartAttributes struct {
		Table []struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			Value      int    `json:"value"`
			Worst      int    `json:"worst"`
			Thresh     int    `json:"thresh"`
			WhenFailed string `json:"when_failed"`
			Raw        struct {
				Value  int64 `json:"value"`
				String string `json:"string"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`
	NvmeSmartHealthInformationLog struct {
		Temperature         int   `json:"temperature"`
		PowerOnHours        int64 `json:"power_on_hours"`
		PowerCycles         int64 `json:"power_cycles"`
		UnsafeShutdowns     int64 `json:"unsafe_shutdowns"`
		MediaErrors         int64 `json:"media_errors"`
		PercentageUsed      int   `json:"percentage_used"`
	} `json:"nvme_smart_health_information_log"`
}

// USBBridgeTypes 支持的 USB 桥接类型
var USBBridgeTypes = []string{"", "sat", "usbsunplus", "usbjmicron", "usbcypress"}

// parseSMARTData 解析 smartctl 输出
func parseSMARTData(deviceName string, usbType string) (*SMARTData, error) {
	args := []string{"--all", "-j"}
	if usbType != "" {
		args = append(args, "-d", usbType)
	}
	args = append(args, deviceName)

	cmd := exec.Command("smartctl", args...)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("smartctl failed: %w", err)
	}

	var raw smartctlOutput
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse smartctl output: %w", err)
	}

	// 检查是否有 USB bridge 错误
	hasUSBError := false
	for _, msg := range raw.Smartctl.Messages {
		if msg.Severity == "error" && strings.Contains(msg.String, "Unknown USB bridge") {
			hasUSBError = true
			break
		}
	}

	if hasUSBError {
		return nil, fmt.Errorf("不支持的 USB 桥接芯片，无法读取 SMART 数据")
	}

	// 构建数据结构
	data := &SMARTData{
		Device: Device{
			Name:    deviceName,
			Model:   raw.ModelName,
			Serial:  raw.SerialNumber,
		},
		SmartStatus: "PASSED",
	}

	if !raw.SmartStatus.Passed {
		data.SmartStatus = "FAILED"
	}

	// 解析设备类型和数据
	if strings.Contains(raw.Device.Protocol, "NVMe") {
		data.Device.DeviceType = "NVMe"
		parseNVMeData(data, &raw)
	} else {
		// ATA/SATA (HDD/SSD)
		parseATAData(data, &raw)
		data.Device.DeviceType = detectDriveType(&raw)
	}

	return data, nil
}

// parseATAData 解析 ATA/SATA 设备数据
func parseATAData(data *SMARTData, raw *smartctlOutput) {
	data.Temperature = raw.Temperature.Current
	data.PowerOnHours = raw.PowerOnTime.Hours
	data.PowerCycleCount = raw.PowerCycleCount

	// 解析 SMART 属性表
	for _, attr := range raw.AtaSmartAttributes.Table {
		data.Attributes = append(data.Attributes, SMARTAttribute{
			ID:         attr.ID,
			Name:       attr.Name,
			Value:      attr.Value,
			Worst:      attr.Worst,
			Threshold:  attr.Thresh,
			RawValue:   attr.Raw.Value,
			WhenFailed: attr.WhenFailed,
		})

		// 提取关键指标
		switch attr.ID {
		case 5: // Reallocated_Sector_Ct
			data.ReallocatedSectors = attr.Raw.Value
		case 196: // Reallocated_Event_Count
			if data.ReallocatedSectors == 0 {
				data.ReallocatedSectors = attr.Raw.Value
			}
		case 197: // Current_Pending_Sector
			data.PendingSectors = attr.Raw.Value
		case 198: // Offline_Uncorrectable
			data.UncorrectableErrors = attr.Raw.Value
		case 194: // Temperature_Celsius
			if data.Temperature == 0 {
				data.Temperature = int(attr.Raw.Value)
			}
		}
	}

	// 计算健康度
	data.HealthPercent = calculateHealth(data)
}

// parseNVMeData 解析 NVMe 设备数据
func parseNVMeData(data *SMARTData, raw *smartctlOutput) {
	log := raw.NvmeSmartHealthInformationLog
	data.Temperature = log.Temperature
	data.PowerOnHours = log.PowerOnHours
	data.PowerCycleCount = log.PowerCycles
	data.UncorrectableErrors = log.MediaErrors
	data.HealthPercent = 100 - log.PercentageUsed
}

// calculateHealth 计算健康度百分比
func calculateHealth(data *SMARTData) int {
	health := 100

	// 重映射扇区每个 -2%
	if data.ReallocatedSectors > 0 {
		health -= int(data.ReallocatedSectors) * 2
	}

	// 待映射扇区每个 -3%
	if data.PendingSectors > 0 {
		health -= int(data.PendingSectors) * 3
	}

	// 不可纠正错误每个 -5%
	if data.UncorrectableErrors > 0 {
		health -= int(data.UncorrectableErrors) * 5
	}

	// SMART 状态失败 -50%
	if data.SmartStatus == "FAILED" {
		health -= 50
	}

	if health < 0 {
		health = 0
	}

	return health
}

// detectDriveType 准确检测 SSD/HDD 类型
func detectDriveType(raw *smartctlOutput) string {
	// 1. 最可靠：rotation_rate = 0 表示 SSD
	if raw.RotationRate == 0 {
		return "SSD"
	}

	// 2. rotation_rate > 0 表示机械硬盘
	if raw.RotationRate > 0 {
		return "HDD"
	}

	// 3. 检查 TRIM 支持（SSD 特性）
	if raw.Trim.Supported {
		return "SSD"
	}

	// 4. 最后才检查型号名称关键词
	model := strings.ToLower(raw.ModelName)
	ssdKeywords := []string{"ssd", "solid state", "nvme"}
	for _, kw := range ssdKeywords {
		if strings.Contains(model, kw) {
			return "SSD"
		}
	}

	// 默认返回 HDD
	return "HDD"
}

// detectDeviceType 根据设备名检测类型
func detectDeviceType(name string) string {
	if strings.Contains(name, "nvme") {
		return "NVMe"
	}
	return "Unknown"
}