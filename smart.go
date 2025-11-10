package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Device 表示一个存储设备
type Device struct {
	Name       string `json:"name"`        // 设备名称 /dev/sda
	Model      string `json:"model"`       // 型号
	Serial     string `json:"serial"`      // 序列号
	DeviceType string `json:"device_type"` // HDD/SSD/NVMe
	CapacityGB int64  `json:"capacity_gb"` // 容量(GB)
	IsExternal bool   `json:"is_external"` // 是否为外置设备
}

// SMARTData 表示 SMART 数据快照
type SMARTData struct {
	Device              Device             `json:"device"`
	Temperature         int                `json:"temperature"`          // 温度 °C
	PowerOnHours        int64              `json:"power_on_hours"`       // 通电时间
	PowerCycleCount     int64              `json:"power_cycle_count"`    // 通电次数
	ReallocatedSectors  int64              `json:"reallocated_sectors"`  // 重映射扇区
	PendingSectors      int64              `json:"pending_sectors"`      // 待映射扇区
	UncorrectableErrors int64              `json:"uncorrectable_errors"` // 不可纠正错误
	HealthPercent       int                `json:"health_percent"`       // 健康度百分比
	SmartStatus         string             `json:"smart_status"`         // PASSED/FAILED
	Attributes          []SMARTAttribute   `json:"attributes"`           // 所有属性
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

// ListDevices 列出所有支持 SMART 的设备
func ListDevices() ([]Device, error) {
	// 首先用 smartctl 扫描
	devices := make(map[string]Device)

	out, err := exec.Command("smartctl", "--scan-open", "-j").Output()
	if err == nil {
		var result struct {
			Devices []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"devices"`
		}

		if err := json.Unmarshal(out, &result); err == nil {
			for _, d := range result.Devices {
				// 跳过 CD/DVD 设备
				if strings.Contains(d.Type, "scsi") && !strings.Contains(d.Name, "sd") {
					continue
				}
				capacity := getDiskCapacity(d.Name)
			isExternal := isExternalEnclosure(d.Name)
			fmt.Printf("ListDevices - %s: capacity = %d GB, external = %v\n", d.Name, capacity, isExternal)
			devices[d.Name] = Device{
				Name:       d.Name,
				DeviceType: detectDeviceType(d.Name),
				CapacityGB: capacity,
				IsExternal: isExternal,
			}
			}
		}
	}

	// Linux: 扫描所有 /dev/sd* 块设备（包括 USB 硬盘）
	if runtime.GOOS == "linux" {
		scanLinuxBlockDevices(devices)
	}

	// 转换为列表
	deviceList := make([]Device, 0, len(devices))
	for _, d := range devices {
		deviceList = append(deviceList, d)
	}

	return deviceList, nil
}

// scanLinuxBlockDevices 扫描 Linux 块设备
func scanLinuxBlockDevices(devices map[string]Device) {
	// 读取 /sys/block/ 目录
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		// 只处理 sd* 设备（SATA/USB 硬盘）
		if !strings.HasPrefix(name, "sd") {
			continue
		}

		devicePath := "/dev/" + name
		if _, exists := devices[devicePath]; exists {
			continue // 已经被 smartctl --scan 检测到
		}

		// 检查是否为分区
		if isPartition(name) {
			continue
		}

		// 尝试读取这个设备
		if canReadSMART(devicePath) {
			capacity := getDiskCapacity(devicePath)
			isExternal := isExternalEnclosure(devicePath)
			devices[devicePath] = Device{
				Name:       devicePath,
				DeviceType: detectDeviceType(devicePath),
				CapacityGB: capacity,
				IsExternal: isExternal,
			}
		}
	}
}

// isPartition 判断是否为分区（如 sda1, sdb2）
func isPartition(name string) bool {
	if len(name) < 4 {
		return false
	}
	// sda1, sdb2 等是分区，sda, sdb 是设备
	lastChar := name[len(name)-1]
	return lastChar >= '0' && lastChar <= '9'
}

// canReadSMART 测试能否读取 SMART 数据
func canReadSMART(devicePath string) bool {
	// 尝试多种 USB 桥接类型
	usbTypes := []string{"", "sat", "usbsunplus", "usbjmicron", "usbcypress"}

	for _, usbType := range usbTypes {
		args := []string{"--all", "-j"}
		if usbType != "" {
			args = append(args, "-d", usbType)
		}
		args = append(args, devicePath)

		cmd := exec.Command("smartctl", args...)
		out, _ := cmd.CombinedOutput()

		var result struct {
			SmartStatus struct {
				Passed bool `json:"passed"`
			} `json:"smart_status"`
			Messages []struct {
				String   string `json:"string"`
				Severity string `json:"severity"`
			} `json:"messages"`
		}

		if err := json.Unmarshal(out, &result); err == nil {
			// 检查是否有严重错误
			hasError := false
			for _, msg := range result.Messages {
				if msg.Severity == "error" && strings.Contains(msg.String, "Unknown USB bridge") {
					hasError = true
					break
				}
			}
			if !hasError {
				return true
			}
		}
	}

	return false
}

// GetSMARTData 获取指定设备的 SMART 数据
func GetSMARTData(deviceName string) (*SMARTData, error) {
	// 尝试不同的 USB 桥接类型
	usbTypes := []string{"", "sat", "usbsunplus", "usbjmicron", "usbcypress"}

	var lastErr error
	for _, usbType := range usbTypes {
		args := []string{"--all", "-j"}
		if usbType != "" {
			args = append(args, "-d", usbType)
		}
		args = append(args, deviceName)

		cmd := exec.Command("smartctl", args...)
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			lastErr = err
			continue
		}

		var raw smartctlOutput
		if err := json.Unmarshal(out, &raw); err != nil {
			lastErr = fmt.Errorf("parse smartctl output: %w", err)
			continue
		}

		// 检查是否有 USB bridge 错误
		hasUSBError := false
		for _, msg := range raw.Smartctl.Messages {
			if msg.Severity == "error" && strings.Contains(msg.String, "Unknown USB bridge") {
				hasUSBError = true
				break
			}
		}

		if hasUSBError && usbType != usbTypes[len(usbTypes)-1] {
			// 还有其他类型可以尝试
			continue
		}

		if hasUSBError {
			return nil, fmt.Errorf("不支持的 USB 桥接芯片，无法读取 SMART 数据")
		}

		// 成功读取
		capacity := getDiskCapacity(deviceName)
		isExternal := isExternalEnclosure(deviceName)
		data := &SMARTData{
			Device: Device{
				Name:       deviceName,
				Model:      raw.ModelName,
				Serial:     raw.SerialNumber,
				DeviceType: detectDeviceType(deviceName),
				CapacityGB: capacity,
				IsExternal: isExternal,
			},
			SmartStatus: "PASSED",
		}

		// 调试日志
		fmt.Printf("Device %s: capacity = %d GB, external = %v\n", deviceName, capacity, isExternal)

		if !raw.SmartStatus.Passed {
			data.SmartStatus = "FAILED"
		}

		// 检测设备类型并解析对应数据
		if strings.Contains(raw.Device.Protocol, "NVMe") {
			data.Device.DeviceType = "NVMe"
			parseNVMeData(data, &raw)
		} else {
			// ATA/SATA (HDD/SSD)
			parseATAData(data, &raw)
			// 使用 rotation_rate 准确判断 SSD
			data.Device.DeviceType = detectDriveType(&raw)
		}

		return data, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("smartctl failed: %w", lastErr)
	}
	return nil, fmt.Errorf("无法读取设备 SMART 数据")
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

	// 计算健康度（简化版）
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

// detectDeviceType 根据设备名检测类型
func detectDeviceType(name string) string {
	if strings.Contains(name, "nvme") {
		return "NVMe"
	}
	return "Unknown"
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

// isSSD 判断是否为 SSD（已废弃，保留用于兼容）
func isSSD(model string) bool {
	model = strings.ToLower(model)
	ssdKeywords := []string{"ssd", "solid state", "nvme"}
	for _, kw := range ssdKeywords {
		if strings.Contains(model, kw) {
			return true
		}
	}
	return false
}

// getDiskCapacity 获取磁盘容量(GB)
func getDiskCapacity(deviceName string) int64 {
	fmt.Printf("DEBUG: getDiskCapacity called with device: %s\n", deviceName)

	// macOS 特殊处理
	if runtime.GOOS == "darwin" {
		// 直接使用原始设备名
		baseName := strings.TrimPrefix(deviceName, "/dev/")

		// 处理 rdiskX 格式 -> diskX
		if strings.HasPrefix(baseName, "rdisk") {
			baseName = strings.TrimPrefix(baseName, "r")
		}

		// 处理分区，如 disk0s1 -> disk0
		if strings.Contains(baseName, "s") {
			parts := strings.Split(baseName, "s")
			if len(parts) > 0 {
				baseName = parts[0]
			}
		}

		fmt.Printf("DEBUG: macOS disk identifier: %s\n", baseName)

		// 使用 diskutil 获取信息
		cmd := exec.Command("diskutil", "info", "/dev/"+baseName)
		out, err := cmd.Output()
		if err != nil {
			fmt.Printf("DEBUG: diskutil error for %s: %v\n", baseName, err)
			return 0
		}

		output := string(out)
		fmt.Printf("DEBUG: diskutil output for %s:\n%s\n", baseName, output)

		// 查找 "Total Size" 行
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Total Size:") {
				fmt.Printf("DEBUG: Found Total Size line: %s\n", line)
				// 解析类似 "Total Size: 500.1 GB (500107862016 Bytes)" 的行
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "GB" && i > 0 {
						if size, parseErr := strconv.ParseFloat(parts[i-1], 64); parseErr == nil {
							capacity := int64(size)
							fmt.Printf("DEBUG: Parsed capacity: %d GB\n", capacity)
							return capacity
						}
					}
					if part == "TB" && i > 0 {
						if size, parseErr := strconv.ParseFloat(parts[i-1], 64); parseErr == nil {
							capacity := int64(size * 1024) // 转换为GB
							fmt.Printf("DEBUG: Parsed capacity: %d GB (from TB)\n", capacity)
							return capacity
						}
					}
				}
			}
		}

		fmt.Printf("DEBUG: Could not find Total Size for %s\n", baseName)
		return 0
	}

	// Linux 处理
	if runtime.GOOS == "linux" {
		// 提取设备名中的 sdX 或 nvmeXn1 部分
		baseName := ""
		if strings.Contains(deviceName, "nvme") {
			// nvme设备如 /dev/nvme0n1p1 -> nvme0n1
			parts := strings.Split(strings.TrimPrefix(deviceName, "/dev/"), "p")
			baseName = parts[0]
		} else {
			// 普通设备如 /dev/sda1 -> sda
			baseName = strings.TrimPrefix(deviceName, "/dev/")
			// 移除分区号
			for i := len(baseName) - 1; i >= 0; i-- {
				if baseName[i] >= '0' && baseName[i] <= '9' {
					baseName = baseName[:i]
				} else {
					break
				}
			}
		}

		sizePath := fmt.Sprintf("/sys/block/%s/size", baseName)
		if data, err := os.ReadFile(sizePath); err == nil {
			if sectors, parseErr := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); parseErr == nil {
				// 扇区大小通常是 512 字节
				capacityGB := (sectors * 512) / (1024 * 1024 * 1024)
				return capacityGB
			}
		}
	}

	// Windows 处理
	if runtime.GOOS == "windows" {
		// 获取磁盘号
		diskNum := "0"
		baseName := strings.TrimPrefix(deviceName, "/dev/")
		if strings.HasPrefix(baseName, "PhysicalDrive") {
			diskNum = strings.TrimPrefix(baseName, "PhysicalDrive")
		}

		cmd := exec.Command("wmic", "diskdrive", "where", fmt.Sprintf("Index=%s", diskNum), "get", "Size")
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 1 {
				if size, parseErr := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64); parseErr == nil {
					capacityGB := size / (1024 * 1024 * 1024)
					return capacityGB
				}
			}
		}
	}

	return 0
}

// isExternalDevice 检测是否为外置设备
func isExternalDevice(deviceName string) bool {
	baseName := strings.TrimPrefix(deviceName, "/dev/")

	// 处理 rdiskX 格式 -> diskX
	if strings.HasPrefix(baseName, "rdisk") {
		baseName = strings.TrimPrefix(baseName, "r")
	}

	// 处理分区，如 disk0s1 -> disk0
	if strings.Contains(baseName, "s") && !strings.HasPrefix(baseName, "disk") {
		parts := strings.Split(baseName, "s")
		if len(parts) > 0 {
			baseName = parts[0]
		}
	}

	fmt.Printf("DEBUG: isExternalDevice checking %s -> %s\n", deviceName, baseName)

	if runtime.GOOS == "darwin" {
		// 使用 diskutil 获取设备信息
		cmd := exec.Command("diskutil", "info", "/dev/"+baseName)
		out, err := cmd.Output()
		if err != nil {
			fmt.Printf("DEBUG: diskutil info failed for %s: %v\n", baseName, err)
			return false
		}

		output := string(out)
		fmt.Printf("DEBUG: diskutil info for %s:\n%s\n", baseName, output)

		lines := strings.Split(output, "\n")
		isExternal := false

		for _, line := range lines {
			line = strings.TrimSpace(line)

			// 检查设备位置（硬盘柜通常在 External 设备位置）
			if strings.Contains(line, "Device Location:") {
				if strings.Contains(line, "External") ||
				   strings.Contains(line, "USB") ||
				   strings.Contains(line, "Thunderbolt") {
					fmt.Printf("DEBUG: Found external location: %s\n", line)
					isExternal = true
				}
			}

			// 检查设备/媒体类型
			if strings.Contains(line, "Device / Media Type:") {
				if strings.Contains(line, "USB") ||
				   strings.Contains(line, "External") ||
				   strings.Contains(line, "Removable") {
					fmt.Printf("DEBUG: Found external media type: %s\n", line)
					isExternal = true
				}
			}

			// 检查协议
			if strings.Contains(line, "Protocol:") {
				if strings.Contains(line, "USB") ||
				   strings.Contains(line, "FireWire") ||
				   strings.Contains(line, "Thunderbolt") {
					fmt.Printf("DEBUG: Found external protocol: %s\n", line)
					isExternal = true
				}
			}

			// 检查设备节点（硬盘柜通常在 USB 或 Thunderbolt 总线上）
			if strings.Contains(line, "Device Node:") {
				if strings.Contains(line, "disk") &&
				   (strings.Contains(line, "usb") ||
				    strings.Contains(line, "thunderbolt")) {
					fmt.Printf("DEBUG: Found external device node: %s\n", line)
					isExternal = true
				}
			}

			// 检查是否为外部设备
			if strings.Contains(line, "External:") && strings.Contains(line, "Yes") {
				fmt.Printf("DEBUG: Found external flag: %s\n", line)
				isExternal = true
			}

			// 检查是否为可移动设备
			if strings.Contains(line, "Removable Media:") && strings.Contains(line, "Yes") {
				fmt.Printf("DEBUG: Found removable media: %s\n", line)
				isExternal = true
			}

			// 检查总线协议
			if strings.Contains(line, "Bus Protocol:") {
				if strings.Contains(line, "USB") ||
				   strings.Contains(line, "Thunderbolt") {
					fmt.Printf("DEBUG: Found external bus protocol: %s\n", line)
					isExternal = true
				}
			}
		}

		fmt.Printf("DEBUG: isExternalDevice result for %s: %v\n", baseName, isExternal)
		return isExternal
	}

	// Linux 处理
	if runtime.GOOS == "linux" {
		// 检查设备路径是否包含 usb 等关键词
		cmd := exec.Command("udevadm", "info", "--query=property", "--name="+deviceName)
		if out, err := cmd.Output(); err == nil {
			output := string(out)
			if strings.Contains(output, "ID_BUS=usb") ||
			   strings.Contains(output, "ID_BUS=firewire") ||
			   strings.Contains(output, "ID_SERIAL=") {
				return true
			}
		}
	}

	// Windows 处理
	if runtime.GOOS == "windows" {
		// 可以通过 wmic 检测 USB 设备
		cmd := exec.Command("wmic", "diskdrive", "where", "InterfaceType='USB'", "get", "DeviceID")
		if out, err := cmd.Output(); err == nil {
			if strings.Contains(string(out), "USB") {
				return true
			}
		}
	}

	return false
}

// isExternalEnclosure 检测是否为外置硬盘柜/硬盘盒
func isExternalEnclosure(deviceName string) bool {
	if runtime.GOOS != "darwin" {
		return false
	}

	baseName := strings.TrimPrefix(deviceName, "/dev/")

	// 处理 rdiskX 格式 -> diskX
	if strings.HasPrefix(baseName, "rdisk") {
		baseName = strings.TrimPrefix(baseName, "r")
	}

	// 处理分区，如 disk0s1 -> disk0
	if strings.Contains(baseName, "s") && !strings.HasPrefix(baseName, "disk") {
		parts := strings.Split(baseName, "s")
		if len(parts) > 0 {
			baseName = parts[0]
		}
	}

	// 使用 diskutil 获取详细设备信息
	cmd := exec.Command("diskutil", "info", "/dev/"+baseName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	output := string(out)

	// 常见硬盘柜品牌和型号关键词
	enclosureKeywords := []string{
		// 硬盘柜品牌
		"ORICO", "Orico", "orico",
		"ICY BOX", "Icy Box", "icybox",
		"QNAP", "qnap", "QNAP",
		"Synology", "synology",
		"WD", "Western Digital", "My Book", "My Passport",
		"Seagate", "Expansion", "Backup Plus",
		"TOSHIBA", "Canvio", "TOSHIBA",
		"HGST", "HGST",
		"LaCie", "lacie",
		"Buffalo", "BUFFALO",
		"Adata", "ADATA",
		"Samsung", "T-series", "T-series",
		"Corsair", "CORSAIR",
		"OWC", "Other World Computing",
		"CalDigit", "caldigit",
		"G-Technology", "G-DRIVE",
		"Promise", "Promise Technology",
		"Drobo", "drobo",
		"ASUSTOR", "asustor",
		"QSAN", "qsan",
		"Thecus", "thecus",
		// 硬盘盒类型
		"Enclosure", "enclosure",
		"External", "external",
		"Dock", "dock",
		"Adapter", "adapter",
		"Bridge", "bridge",
		// USB 转接芯片品牌
		"JMicron", "jmicron", "JMS",
		"ASMedia", "asmedia", "ASM",
		"Initio", "initio",
		"Sunplus", "sunplus", "SPIF",
		"LucidPort", "lucidport",
		"Phison", "phison",
		"Realtek", "realtek", "RTL",
		// 其他关键词
		"USB", "usb",
		"Thunderbolt", "thunderbolt", "TB",
		"FireWire", "firewire", "FW",
		"eSATA", "esata",
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 检查设备型号和产品名称
		if strings.Contains(line, "Device / Media Name:") ||
		   strings.Contains(line, "Product Name:") ||
		   strings.Contains(line, "Model:") ||
		   strings.Contains(line, "Vendor:") {
			for _, keyword := range enclosureKeywords {
				if strings.Contains(strings.ToLower(line), strings.ToLower(keyword)) {
					fmt.Printf("DEBUG: Found enclosure keyword in product/model: %s\n", line)
					return true
				}
			}
		}

		// 检查卷名称
		if strings.Contains(line, "Volume Name:") {
			for _, keyword := range enclosureKeywords {
				if strings.Contains(strings.ToLower(line), strings.ToLower(keyword)) {
					fmt.Printf("DEBUG: Found enclosure keyword in volume name: %s\n", line)
					return true
				}
			}
		}
	}

	// 如果上述检测都没有结果，使用通用外置设备检测
	return isExternalDevice(deviceName)
}

// CheckSmartctlInstalled 检查 smartctl 是否安装
func CheckSmartctlInstalled() error {
	_, err := exec.LookPath("smartctl")
	if err != nil {
		msg := "smartctl not found. Please install smartmontools:\n"
		switch runtime.GOOS {
		case "darwin":
			msg += "  brew install smartmontools"
		case "linux":
			msg += "  sudo apt install smartmontools  # Debian/Ubuntu\n"
			msg += "  sudo yum install smartmontools  # RedHat/CentOS"
		case "windows":
			msg += "  Download from: https://www.smartmontools.org/"
		}
		return fmt.Errorf(msg)
	}
	return nil
}
