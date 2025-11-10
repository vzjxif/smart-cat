package handler

import (
	"embed"
	"net/http"
)

// DeviceHandler 设备相关处理器
type DeviceHandler struct {
	*Handler
	webFiles embed.FS
}

// NewDeviceHandler 创建设备处理器
func NewDeviceHandler(handler *Handler, webFiles embed.FS) *DeviceHandler {
	return &DeviceHandler{
		Handler:  handler,
		webFiles: webFiles,
	}
}

// HandleIndex 首页处理器
func (h *DeviceHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := h.webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// HandleDevices 获取设备列表
func (h *DeviceHandler) HandleDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.deviceService.GetAllDevices()
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, devices)
}

// HandleSmart 获取指定设备的实时 SMART 数据
func (h *DeviceHandler) HandleSmart(w http.ResponseWriter, r *http.Request) {
	deviceName := parseDevicePath(r)
	if deviceName == "" {
		h.respondError(w, http.StatusBadRequest, "device name required")
		return
	}

	data, err := h.deviceService.GetSMARTData(deviceName)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, data)
}

// HandleHistory 获取历史数据
func (h *DeviceHandler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	serial := parseSerialPath(r)
	if serial == "" {
		h.respondError(w, http.StatusBadRequest, "serial number required")
		return
	}

	from, to, err := parseTimeRange(r)
	if err != nil {
		if fromStr := r.URL.Query().Get("from"); fromStr != "" {
			h.respondError(w, http.StatusBadRequest, "invalid from time format")
			return
		}
		if toStr := r.URL.Query().Get("to"); toStr != "" {
			h.respondError(w, http.StatusBadRequest, "invalid to time format")
			return
		}
	}

	records, err := h.deviceService.GetHistory(serial, from, to)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, records)
}