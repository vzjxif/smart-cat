package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"smart-cat/internal/config"
	"smart-cat/internal/handler"
	"smart-cat/internal/service"
	"smart-cat/internal/smart"
	"smart-cat/internal/storage"
)

//go:embed web
var webFiles embed.FS

func main() {
	// 加载配置
	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	// 检查系统依赖
	detector := smart.NewDeviceDetector()
	if err := detector.CheckSmartctlInstalled(); err != nil {
		log.Fatal(err)
	}

	// 初始化存储层
	store, err := storage.NewCSVStorage(cfg.Collector.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// 初始化服务层
	deviceService := service.NewDeviceService(detector, store)
	collectorConfig := smart.DefaultCollectorConfig()
	collectorConfig.Interval = cfg.Collector.Interval
	collectorConfig.DataDir = cfg.Collector.DataDir
	collectorConfig.Enabled = cfg.Collector.Enabled
	collector := service.NewCollector(detector, store, collectorConfig)

	// 启动后台采集器
	go collector.Start()

	// 初始化HTTP处理器
	h := handler.NewHandler(deviceService)
	deviceHandler := handler.NewDeviceHandler(h, webFiles)

	// 设置路由
	setupRoutes(deviceHandler)

	// 启动服务器
	startServer(cfg.Server.Addr, collector)
}

// setupRoutes 设置路由
func setupRoutes(deviceHandler *handler.DeviceHandler) {
	http.HandleFunc("/", deviceHandler.HandleIndex)
	http.HandleFunc("/api/devices", deviceHandler.HandleDevices)
	http.HandleFunc("/api/smart/", deviceHandler.HandleSmart)
	http.HandleFunc("/api/history/", deviceHandler.HandleHistory)
}

// startServer 启动HTTP服务器
func startServer(addr string, collector *service.Collector) {
	log.Printf("Server starting on http://localhost%s", addr)
	log.Printf("Press Ctrl+C to stop")

	// 优雅退出
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("\nShutting down...")
		collector.Stop()
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}