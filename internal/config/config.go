package config

import (
	"time"
)

// Config 应用配置
type Config struct {
	Server ServerConfig `json:"server"`
	Collector CollectorConfig `json:"collector"`
}

// ServerConfig HTTP服务器配置
type ServerConfig struct {
	Addr string `json:"addr"`
}

// CollectorConfig 数据采集器配置
type CollectorConfig struct {
	Interval time.Duration `json:"interval"`
	DataDir  string        `json:"data_dir"`
	Enabled  bool          `json:"enabled"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr: ":10044",
		},
		Collector: CollectorConfig{
			Interval: time.Hour,
			DataDir:  "./data",
			Enabled:  true,
		},
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		c.Server.Addr = ":10044"
	}
	if c.Collector.Interval <= 0 {
		c.Collector.Interval = time.Hour
	}
	if c.Collector.DataDir == "" {
		c.Collector.DataDir = "./data"
	}
	return nil
}