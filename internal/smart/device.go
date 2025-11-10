package smart

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"smart-cat/pkg/osutils"
)

// DeviceDetector 设备检测器
type DeviceDetector struct{}

// NewDeviceDetector 创建设备检测器
func NewDeviceDetector() *DeviceDetector {
	return &DeviceDetector{}
}

// ListDevices 列出所有支持 SMART 的设备
func (d *DeviceDetector) ListDevices() ([]Device, error) {
	devices := make(map[string]Device)

	// 首先用 smartctl 扫描
	out, err := exec.Command("smartctl", "--scan-open", "-j").Output()
	if err == nil {
		var result struct {
			Devices []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"devices"`
		}

		if err := json.Unmarshal(out, &result); err == nil {
			for _, device := range result.Devices {
				// 跳过 CD/DVD 设备
				if strings.Contains(device.Type, "scsi") && !strings.Contains(device.Name, "sd") {
					continue
				}

				capacity := osutils.GetDiskCapacity(device.Name)
				isExternal := osutils.IsExternalEnclosure(device.Name)

				devices[device.Name] = Device{
					Name:       device.Name,
					DeviceType: detectDeviceType(device.Name),
					CapacityGB: capacity,
					IsExternal: isExternal,
				}
			}
		}
	}

	// Linux: 扫描所有 /dev/sd* 块设备（包括 USB 硬盘）
	if runtime.GOOS == "linux" {
		d.scanLinuxBlockDevices(devices)
	}

	// 转换为列表
	deviceList := make([]Device, 0, len(devices))
	for _, device := range devices {
		deviceList = append(deviceList, device)
	}

	return deviceList, nil
}

// GetSMARTData 获取指定设备的 SMART 数据
func (d *DeviceDetector) GetSMARTData(deviceName string) (*SMARTData, error) {
	// 尝试不同的 USB 桥接类型
	var lastErr error
	for _, usbType := range USBBridgeTypes {
		data, err := parseSMARTData(deviceName, usbType)
		if err != nil {
			lastErr = err
			continue
		}

		// 补充设备信息
		capacity := osutils.GetDiskCapacity(deviceName)
		isExternal := osutils.IsExternalEnclosure(deviceName)
		data.Device.CapacityGB = capacity
		data.Device.IsExternal = isExternal
		data.Timestamp = data.Timestamp // 这里应该设置实际时间

		return data, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("smartctl failed: %w", lastErr)
	}
	return nil, fmt.Errorf("无法读取设备 SMART 数据")
}

// CanReadSMART 测试能否读取 SMART 数据
func (d *DeviceDetector) CanReadSMART(devicePath string) bool {
	for _, usbType := range USBBridgeTypes {
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

// CheckSmartctlInstalled 检查 smartctl 是否安装
func (d *DeviceDetector) CheckSmartctlInstalled() error {
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

// scanLinuxBlockDevices 扫描 Linux 块设备
func (d *DeviceDetector) scanLinuxBlockDevices(devices map[string]Device) {
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
		if d.CanReadSMART(devicePath) {
			capacity := osutils.GetDiskCapacity(devicePath)
			isExternal := osutils.IsExternalEnclosure(devicePath)
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