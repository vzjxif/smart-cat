package osutils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// GetDiskCapacity 获取磁盘容量(GB)
func GetDiskCapacity(deviceName string) int64 {
	// macOS 特殊处理
	if runtime.GOOS == "darwin" {
		return getMacOSDiskCapacity(deviceName)
	}

	// Linux 处理
	if runtime.GOOS == "linux" {
		return getLinuxDiskCapacity(deviceName)
	}

	// Windows 处理
	if runtime.GOOS == "windows" {
		return getWindowsDiskCapacity(deviceName)
	}

	return 0
}

// IsExternalEnclosure 检测是否为外置硬盘柜/硬盘盒
func IsExternalEnclosure(deviceName string) bool {
	if runtime.GOOS == "darwin" {
		return isMacOSExternalEnclosure(deviceName)
	}

	if runtime.GOOS == "linux" {
		return isLinuxExternalDevice(deviceName)
	}

	if runtime.GOOS == "windows" {
		return isWindowsExternalDevice(deviceName)
	}

	return false
}

// getMacOSDiskCapacity macOS 获取磁盘容量
func getMacOSDiskCapacity(deviceName string) int64 {
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

	// 使用 diskutil 获取信息
	cmd := exec.Command("diskutil", "info", "/dev/"+baseName)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	output := string(out)

	// 查找 "Total Size" 行
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Total Size:") {
			// 解析类似 "Total Size: 500.1 GB (500107862016 Bytes)" 的行
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "GB" && i > 0 {
					if size, parseErr := strconv.ParseFloat(parts[i-1], 64); parseErr == nil {
						return int64(size)
					}
				}
				if part == "TB" && i > 0 {
					if size, parseErr := strconv.ParseFloat(parts[i-1], 64); parseErr == nil {
						return int64(size * 1024) // 转换为GB
					}
				}
			}
		}
	}

	return 0
}

// getLinuxDiskCapacity Linux 获取磁盘容量
func getLinuxDiskCapacity(deviceName string) int64 {
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

	return 0
}

// getWindowsDiskCapacity Windows 获取磁盘容量
func getWindowsDiskCapacity(deviceName string) int64 {
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

	return 0
}

// isMacOSExternalEnclosure macOS 检测外置硬盘柜
func isMacOSExternalEnclosure(deviceName string) bool {
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
					return true
				}
			}
		}

		// 检查卷名称
		if strings.Contains(line, "Volume Name:") {
			for _, keyword := range enclosureKeywords {
				if strings.Contains(strings.ToLower(line), strings.ToLower(keyword)) {
					return true
				}
			}
		}
	}

	// 如果上述检测都没有结果，使用通用外置设备检测
	return isMacOSExternalDevice(baseName)
}

// isMacOSExternalDevice macOS 检测外置设备
func isMacOSExternalDevice(baseName string) bool {
	cmd := exec.Command("diskutil", "info", "/dev/"+baseName)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	output := string(out)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 检查设备位置（外置设备通常在 External 设备位置）
		if strings.Contains(line, "Device Location:") {
			if strings.Contains(line, "External") ||
				strings.Contains(line, "USB") ||
				strings.Contains(line, "Thunderbolt") {
				return true
			}
		}

		// 检查设备/媒体类型
		if strings.Contains(line, "Device / Media Type:") {
			if strings.Contains(line, "USB") ||
				strings.Contains(line, "External") ||
				strings.Contains(line, "Removable") {
				return true
			}
		}

		// 检查协议
		if strings.Contains(line, "Protocol:") {
			if strings.Contains(line, "USB") ||
				strings.Contains(line, "FireWire") ||
				strings.Contains(line, "Thunderbolt") {
				return true
			}
		}

		// 检查是否为外部设备
		if strings.Contains(line, "External:") && strings.Contains(line, "Yes") {
			return true
		}

		// 检查是否为可移动设备
		if strings.Contains(line, "Removable Media:") && strings.Contains(line, "Yes") {
			return true
		}

		// 检查总线协议
		if strings.Contains(line, "Bus Protocol:") {
			if strings.Contains(line, "USB") ||
				strings.Contains(line, "Thunderbolt") {
				return true
			}
		}
	}

	return false
}

// isLinuxExternalDevice Linux 检测外置设备
func isLinuxExternalDevice(deviceName string) bool {
	cmd := exec.Command("udevadm", "info", "--query=property", "--name="+deviceName)
	if out, err := cmd.Output(); err == nil {
		output := string(out)
		if strings.Contains(output, "ID_BUS=usb") ||
			strings.Contains(output, "ID_BUS=firewire") ||
			strings.Contains(output, "ID_SERIAL=") {
			return true
		}
	}
	return false
}

// isWindowsExternalDevice Windows 检测外置设备
func isWindowsExternalDevice(deviceName string) bool {
	cmd := exec.Command("wmic", "diskdrive", "where", "InterfaceType='USB'", "get", "DeviceID")
	if out, err := cmd.Output(); err == nil {
		if strings.Contains(string(out), "USB") {
			return true
		}
	}
	return false
}