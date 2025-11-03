package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed web/*
var webFiles embed.FS

var (
	storage        *Storage
	collectionTick = time.Hour // 每小时采集一次
)

func main() {
	// 检查 smartctl 是否安装
	if err := CheckSmartctlInstalled(); err != nil {
		log.Fatal(err)
	}

	// 初始化存储
	var err error
	storage, err = NewStorage("./data")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// 启动后台采集器
	go backgroundCollector()

	// 设置路由
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/devices", handleDevices)
	http.HandleFunc("/api/smart/", handleSmart)
	http.HandleFunc("/api/history/", handleHistory)

	// 启动服务器
	addr := ":10044"
	log.Printf("Server starting on http://localhost%s", addr)
	log.Printf("Press Ctrl+C to stop")

	// 优雅退出
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("\nShutting down...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// backgroundCollector 后台采集器
func backgroundCollector() {
	ticker := time.NewTicker(collectionTick)
	defer ticker.Stop()

	// 启动时立即采集一次
	collectAll()

	for range ticker.C {
		collectAll()
	}
}

// collectAll 采集所有设备的 SMART 数据
func collectAll() {
	devices, err := ListDevices()
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		return
	}

	for _, device := range devices {
		data, err := GetSMARTData(device.Name)
		if err != nil {
			log.Printf("Failed to get SMART data for %s: %v", device.Name, err)
			continue
		}

		if err := storage.SaveRecord(data.Device.Serial, data); err != nil {
			log.Printf("Failed to save record for %s: %v", device.Name, err)
		} else {
			log.Printf("Collected data for %s (S/N: %s)", device.Name, data.Device.Serial)
		}
	}
}

// handleIndex 首页
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleDevices 获取设备列表
func handleDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := ListDevices()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 获取每个设备的基本信息
	type DeviceInfo struct {
		Device
		HasHistory   bool   `json:"has_history"`
		Error        string `json:"error,omitempty"`        // 错误信息
		ErrorMessage string `json:"error_message,omitempty"` // 用户友好的错误消息
	}

	var deviceInfos []DeviceInfo
	serials, _ := storage.GetAllSerials()
	serialMap := make(map[string]bool)
	for _, s := range serials {
		serialMap[s] = true
	}

	for _, device := range devices {
		data, err := GetSMARTData(device.Name)
		if err != nil {
			// 添加无法读取的设备信息
			deviceInfos = append(deviceInfos, DeviceInfo{
				Device: Device{
					Name:       device.Name,
					Model:      "无法读取",
					Serial:     "unknown",
					DeviceType: "Unknown",
				},
				HasHistory:   false,
				Error:        err.Error(),
				ErrorMessage: "无法读取 SMART 数据（可能是不支持的 USB 桥接芯片）",
			})
			log.Printf("Warning: Cannot read SMART data for %s: %v", device.Name, err)
			continue
		}

		deviceInfos = append(deviceInfos, DeviceInfo{
			Device:     data.Device,
			HasHistory: serialMap[data.Device.Serial],
		})
	}

	respondJSON(w, deviceInfos)
}

// handleSmart 获取指定设备的实时 SMART 数据
func handleSmart(w http.ResponseWriter, r *http.Request) {
	deviceName := strings.TrimPrefix(r.URL.Path, "/api/smart/")
	if deviceName == "" {
		respondError(w, http.StatusBadRequest, "device name required")
		return
	}

	// URL decode
	deviceName = strings.ReplaceAll(deviceName, "%2F", "/")

	data, err := GetSMARTData(deviceName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, data)
}

// handleHistory 获取历史数据
func handleHistory(w http.ResponseWriter, r *http.Request) {
	serial := strings.TrimPrefix(r.URL.Path, "/api/history/")
	if serial == "" {
		respondError(w, http.StatusBadRequest, "serial number required")
		return
	}

	// 解析时间参数
	var from, to time.Time
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		var err error
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid from time")
			return
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		var err error
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid to time")
			return
		}
	}

	// 默认最近7天
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -7)
	}

	records, err := storage.GetHistory(serial, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, records)
}

// respondJSON 返回 JSON 响应
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
