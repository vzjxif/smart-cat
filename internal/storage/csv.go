package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"smart-cat/internal/smart"
)

// CSVStorage CSV 文件存储实现
type CSVStorage struct {
	dataDir string
	mu      sync.Mutex
}

// NewCSVStorage 创建 CSV 存储实例
func NewCSVStorage(dataDir string) (*CSVStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	return &CSVStorage{
		dataDir: dataDir,
	}, nil
}

// SaveRecord 实现 Storage 接口
func (s *CSVStorage) SaveRecord(serial string, data *smart.SMARTData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serial == "" {
		serial = "unknown"
	}

	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.csv", serial))

	// 检查文件是否存在，不存在则创建并写入头部
	needHeader := false
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		needHeader = true
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入头部
	if needHeader {
		header := []string{
			"timestamp",
			"temperature",
			"power_on_hours",
			"power_cycle_count",
			"reallocated_sectors",
			"pending_sectors",
			"uncorrectable_errors",
			"health_percent",
		}
		if err := writer.Write(header); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}

	// 写入数据行
	record := []string{
		data.Timestamp.Format(time.RFC3339),
		strconv.Itoa(data.Temperature),
		strconv.FormatInt(data.PowerOnHours, 10),
		strconv.FormatInt(data.PowerCycleCount, 10),
		strconv.FormatInt(data.ReallocatedSectors, 10),
		strconv.FormatInt(data.PendingSectors, 10),
		strconv.FormatInt(data.UncorrectableErrors, 10),
		strconv.Itoa(data.HealthPercent),
	}

	if err := writer.Write(record); err != nil {
		return fmt.Errorf("write record: %w", err)
	}

	return nil
}

// GetHistory 实现 Storage 接口
func (s *CSVStorage) GetHistory(serial string, from, to time.Time) ([]smart.HistoryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if serial == "" {
		serial = "unknown"
	}

	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.csv", serial))

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []smart.HistoryRecord{}, nil
		}
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// 跳过头部
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	var records []smart.HistoryRecord

	for {
		row, err := reader.Read()
		if err != nil {
			break
		}

		if len(row) < 8 {
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, row[0])
		if err != nil {
			continue
		}

		// 时间过滤
		if !from.IsZero() && timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && timestamp.After(to) {
			continue
		}

		temp, _ := strconv.Atoi(row[1])
		powerOnHours, _ := strconv.ParseInt(row[2], 10, 64)
		powerCycleCount, _ := strconv.ParseInt(row[3], 10, 64)
		reallocated, _ := strconv.ParseInt(row[4], 10, 64)
		pending, _ := strconv.ParseInt(row[5], 10, 64)
		uncorrectable, _ := strconv.ParseInt(row[6], 10, 64)
		health, _ := strconv.Atoi(row[7])

		records = append(records, smart.HistoryRecord{
			Timestamp:           timestamp,
			Temperature:         temp,
			PowerOnHours:        powerOnHours,
			PowerCycleCount:     powerCycleCount,
			ReallocatedSectors:  reallocated,
			PendingSectors:      pending,
			UncorrectableErrors: uncorrectable,
			HealthPercent:       health,
		})
	}

	return records, nil
}

// GetAllSerials 实现 Storage 接口
func (s *CSVStorage) GetAllSerials() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var serials []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".csv" {
			serial := name[:len(name)-4] // 移除 .csv 后缀
			serials = append(serials, serial)
		}
	}

	return serials, nil
}

// CleanOldRecords 实现 Storage 接口
func (s *CSVStorage) CleanOldRecords(days int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days)

	serials, err := s.GetAllSerials()
	if err != nil {
		return err
	}

	for _, serial := range serials {
		filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.csv", serial))

		records, err := s.GetHistory(serial, cutoff, time.Time{})
		if err != nil {
			continue
		}

		// 重写文件，只保留新记录
		if err := s.rewriteFile(filename, records); err != nil {
			return err
		}
	}

	return nil
}

// rewriteFile 重写文件（内部方法）
func (s *CSVStorage) rewriteFile(filename string, records []smart.HistoryRecord) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入头部
	header := []string{
		"timestamp",
		"temperature",
		"power_on_hours",
		"power_cycle_count",
		"reallocated_sectors",
		"pending_sectors",
		"uncorrectable_errors",
		"health_percent",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// 写入记录
	for _, rec := range records {
		row := []string{
			rec.Timestamp.Format(time.RFC3339),
			strconv.Itoa(rec.Temperature),
			strconv.FormatInt(rec.PowerOnHours, 10),
			strconv.FormatInt(rec.PowerCycleCount, 10),
			strconv.FormatInt(rec.ReallocatedSectors, 10),
			strconv.FormatInt(rec.PendingSectors, 10),
			strconv.FormatInt(rec.UncorrectableErrors, 10),
			strconv.Itoa(rec.HealthPercent),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}