package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"smart-cat/internal/service"
)

// Handler HTTP处理器集合
type Handler struct {
	deviceService *service.DeviceService
}

// NewHandler 创建处理器
func NewHandler(deviceService *service.DeviceService) *Handler {
	return &Handler{
		deviceService: deviceService,
	}
}

// respondJSON 返回 JSON 响应
func (h *Handler) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func (h *Handler) respondError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// parseDevicePath 从URL路径解析设备名称
func parseDevicePath(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/api/smart/")
	// URL decode
	path = strings.ReplaceAll(path, "%2F", "/")
	return path
}

// parseSerialPath 从URL路径解析序列号
func parseSerialPath(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, "/api/history/")
}

// parseTimeRange 解析时间范围参数
func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	var from, to time.Time
	var err error

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return from, to, nil
}