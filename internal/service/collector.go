package service

import (
	"log"
	"time"

	"smart-cat/internal/smart"
	"smart-cat/internal/storage"
)

// Collector 数据采集服务
type Collector struct {
	detector *smart.DeviceDetector
	storage  storage.Storage
	config   *smart.CollectorConfig
	ticker   *time.Ticker
	stopChan chan struct{}
}

// NewCollector 创建数据采集服务
func NewCollector(detector *smart.DeviceDetector, storage storage.Storage, config *smart.CollectorConfig) *Collector {
	return &Collector{
		detector: detector,
		storage:  storage,
		config:   config,
		stopChan: make(chan struct{}),
	}
}

// Start 启动采集器
func (c *Collector) Start() {
	if !c.config.Enabled {
		log.Println("Collector is disabled")
		return
	}

	log.Printf("Starting collector with interval %v", c.config.Interval)

	c.ticker = time.NewTicker(c.config.Interval)
	defer c.ticker.Stop()

	// 启动时立即采集一次
	c.collectAll()

	for {
		select {
		case <-c.ticker.C:
			c.collectAll()
		case <-c.stopChan:
			log.Println("Collector stopped")
			return
		}
	}
}

// Stop 停止采集器
func (c *Collector) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	close(c.stopChan)
}

// collectAll 采集所有设备的 SMART 数据
func (c *Collector) collectAll() {
	log.Println("Starting SMART data collection...")

	devices, err := c.detector.ListDevices()
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		return
	}

	successCount := 0
	for _, device := range devices {
		data, err := c.detector.GetSMARTData(device.Name)
		if err != nil {
			log.Printf("Failed to get SMART data for %s: %v", device.Name, err)
			continue
		}

		// 设置采集时间
		data.Timestamp = time.Now()

		if err := c.storage.SaveRecord(data.Device.Serial, data); err != nil {
			log.Printf("Failed to save record for %s: %v", device.Name, err)
		} else {
			log.Printf("Collected data for %s (S/N: %s)", device.Name, data.Device.Serial)
			successCount++
		}
	}

	log.Printf("Collection completed. Successfully collected %d/%d devices", successCount, len(devices))
}

// SetConfig 更新配置
func (c *Collector) SetConfig(config *smart.CollectorConfig) {
	c.config = config

	// 如果正在运行，重新启动以应用新配置
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = time.NewTicker(c.config.Interval)
	}
}