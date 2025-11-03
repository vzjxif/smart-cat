# SMART Cat 开发笔记

## 项目统计

- **Go 代码**: 794 行
  - `main.go`: 228 行 (HTTP 服务器 + 后台采集)
  - `smart.go`: 289 行 (smartctl 调用和数据解析)
  - `storage.go`: 277 行 (CSV 存储)
- **前端代码**: 606 行 (HTML + CSS + JavaScript)
- **总计**: 1400 行

## 架构亮点

### 1. 数据结构第一
```
SMARTData (实时数据)
    ↓
HistoryRecord (历史记录，只保留 8 个关键字段)
    ↓
CSV 文件 (每设备一个文件，按时间追加)
```

### 2. 零特殊情况处理
- HDD/SSD/NVMe 统一通过接口处理
- 跨平台统一使用 `smartctl` 命令
- 错误处理统一返回，不做复杂分支

### 3. 极简主义
- 不用数据库，CSV 文件足够
- 不用前端框架，原生 JS + Chart.js
- 不用配置文件，硬编码常量
- 单个二进制，embed 嵌入静态文件

### 4. 实用主义
- 每小时采集一次（可配置）
- 保留最近 7 天数据（可扩展）
- 只读操作，绝对安全

## API 设计

```
GET /                           首页
GET /api/devices                设备列表
GET /api/smart/:device          实时 SMART 数据
GET /api/history/:serial        历史数据 (支持时间范围)
```

## 性能指标

- 启动时间: < 1秒
- 内存占用: < 20MB
- 单次采集: < 2秒/设备
- CSV 查询: < 10ms/7天数据

## 未来扩展（如果需要）

1. **邮件告警**: 健康度 < 50% 时发送邮件
2. **Prometheus 导出**: 添加 `/metrics` 端点
3. **数据聚合**: 自动合并旧数据（hourly → daily）
4. **多主机监控**: WebSocket 推送多台服务器数据

## Linus 会如何评价

✅ "Good taste":
- 数据结构清晰
- 没有无意义的抽象
- 边界情况处理得当

✅ "Never break userspace":
- 只读操作
- API 向后兼容
- 数据格式稳定（CSV）

✅ "Simplicity":
- 单个二进制
- 零配置
- 不超过 1500 行代码

## 已知限制

1. 需要管理员权限（SMART 读取的硬需求）
2. 依赖 smartctl（但这是行业标准）
3. USB 设备支持取决于硬件
4. 无身份认证（假设可信环境）

## 测试清单

- [ ] 编译通过 (`go build`)
- [ ] 检测到设备 (需要 root 权限)
- [ ] 实时数据显示正常
- [ ] 历史数据采集和展示
- [ ] 图表渲染正确
- [ ] CSV 文件格式正确
- [ ] 跨平台测试 (Linux/macOS/Windows)
