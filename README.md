# SMART Cat 🐱

一个简洁、现代化的本地硬盘 SMART 监控工具。用 Go 编写，通过 Web 界面实时展示硬盘健康状况和历史趋势。

## 特性

- ✅ **跨平台支持** - Linux、macOS、Windows
- ✅ **实时监控** - 温度、健康度、错误计数等关键指标
- ✅ **历史趋势** - 自动记录并展示历史数据，提前发现劣化趋势
- ✅ **智能 USB 检测** - 自动尝试多种 USB 桥接类型，最大化设备兼容性
- ✅ **现代化界面** - 渐变设计、响应式布局、图表可视化
- ✅ **Docker 支持** - 一键部署，无需配置环境
- ✅ **零依赖运行** - 单个二进制文件，开箱即用
- ✅ **轻量存储** - 使用 CSV 文件存储历史数据，无需数据库

## 前置要求

### Docker 部署（推荐 - 仅 Linux）

**⚠️ 重要提示**：
- ✅ **Linux**: Docker 完美支持，推荐使用
- ❌ **macOS**: Docker Desktop 无法访问物理硬盘（架构限制），请使用本地部署
- ⚠️ **Windows**: Docker Desktop 需要 WSL2，设备访问受限，推荐本地部署

只需安装 Docker：
```bash
# Linux
curl -fsSL https://get.docker.com | sh
```

### 本地部署（macOS/Windows 推荐）

必须安装 `smartmontools` 工具：

### macOS（推荐）
```bash
brew install smartmontools
```

**为什么 macOS 不能用 Docker？**
- Docker Desktop for Mac 运行在 Linux 虚拟机中
- 虚拟机无法访问 macOS 的物理硬盘设备（`/dev/diskX`）
- 这是 Docker Desktop 的架构限制，无法绕过

### Linux (Debian/Ubuntu)
```bash
sudo apt install smartmontools
```

### Linux (RedHat/CentOS)
```bash
sudo yum install smartmontools
```

### Windows
从 [smartmontools 官网](https://www.smartmontools.org/) 下载安装。

## 安装

### 方式一：Docker（推荐）

**最简单的方式，无需安装 Go 和 smartmontools**

```bash
# 使用 docker-compose（推荐）
docker-compose up -d

# 或使用 docker run
docker run -d \
  --name smart-cat \
  --privileged \
  -p 8080:8080 \
  -v ./data:/app/data \
  -v /sys:/sys:ro \
  smart-cat:latest
```

**更安全的方式（只挂载特定设备）**:

```bash
docker run -d \
  --name smart-cat \
  --device=/dev/sda \
  --device=/dev/sdb \
  --device=/dev/sdc \
  -p 8080:8080 \
  -v ./data:/app/data \
  -v /sys:/sys:ro \
  smart-cat:latest
```

### 方式二：从源码编译

```bash
git clone <repository>
cd smart-cat
go build
```

### 方式三：直接运行

```bash
go run .
```

## 运行

### Docker

```bash
# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down

# 重启
docker-compose restart
```

### macOS/Linux

需要 root 权限读取 SMART 数据：

```bash
sudo ./smart-cat
```

### Windows

以管理员身份运行：

```powershell
smart-cat.exe
```

程序启动后，访问 http://localhost:8080

## 使用说明

### 主界面

- 显示所有检测到的硬盘设备
- 实时显示健康度、温度、通电时间等关键指标
- 点击设备卡片查看详细信息

### 详情页

- 完整的 SMART 属性列表
- 历史数据趋势图（温度、健康度）
- 支持查看最近 7 天的数据

### 数据采集

- 程序启动时立即采集一次
- 之后每小时自动采集一次
- 数据存储在 `./data/` 目录下，每个设备一个 CSV 文件

## 架构设计

### 核心原则

遵循 Linus Torvalds 的 "Good Taste" 哲学：

1. **数据结构第一** - 用最简单的 CSV 文件存储时序数据
2. **消除特殊情况** - 统一的接口处理不同类型设备（HDD/SSD/NVMe）
3. **保持简洁** - 整个项目不超过 600 行代码
4. **实用主义** - 解决真实问题，不过度设计

### 文件结构

```
smart-cat/
├── main.go       # HTTP 服务器 + 后台采集调度
├── smart.go      # smartctl 调用和 SMART 数据解析
├── storage.go    # CSV 历史数据存储
├── web/
│   └── index.html  # 前端界面（嵌入到二进制）
└── data/         # 运行时创建，存储 CSV 历史数据
    └── <serial>.csv
```

### API 端点

| 端点 | 说明 |
|------|------|
| `GET /` | 主页面 |
| `GET /api/devices` | 获取所有设备列表 |
| `GET /api/smart/:device` | 获取指定设备的实时 SMART 数据 |
| `GET /api/history/:serial?from=&to=` | 获取历史数据 |

### CSV 格式

每个设备一个文件，文件名为序列号：

```csv
timestamp,temperature,power_on_hours,power_cycle_count,reallocated_sectors,pending_sectors,uncorrectable_errors,health_percent
2025-11-03T10:00:00Z,42,15234,100,0,0,0,100
2025-11-03T11:00:00Z,43,15235,100,0,0,0,100
```

## 健康度计算

简化的健康度评分算法：

- 基础分：100
- 重映射扇区：每个 -2%
- 待映射扇区：每个 -3%
- 不可纠正错误：每个 -5%
- SMART 状态失败：-50%

## 常见问题

### 1. Docker: 为什么需要 privileged 模式？

**问题**: Docker 容器需要访问主机的 `/dev/sdX` 设备

**解决方案**:

**方式一（推荐）**: 只挂载需要的设备
```bash
docker run -d \
  --device=/dev/sda \
  --device=/dev/sdb \
  -p 8080:8080 \
  smart-cat:latest
```

**方式二**: 使用 privileged 模式（简单但安全性较低）
```bash
docker run -d \
  --privileged \
  -p 8080:8080 \
  smart-cat:latest
```

### 2. Docker: 历史数据如何持久化？

**问题**: 容器重启后历史数据丢失

**解决**: 挂载数据目录
```bash
docker-compose up -d  # 自动挂载 ./data

# 或手动挂载
docker run -d \
  -v $(pwd)/data:/app/data \
  smart-cat:latest
```

### 3. macOS: Docker 运行后无数据

**问题**: 在 macOS 上运行 `docker-compose up -d` 后，页面显示"未检测到支持 SMART 的设备"

**根本原因**: Docker Desktop for Mac 的架构限制
- Docker 运行在 Linux 虚拟机中
- 虚拟机无法访问 macOS 物理硬盘
- 这是 Docker Desktop 的设计限制

**解决方案**: **不使用 Docker**，直接在 macOS 上运行

```bash
# 1. 停止 Docker 容器
docker-compose down

# 2. 安装依赖
brew install smartmontools

# 3. 编译运行
go build
sudo ./smart-cat
```

然后访问 http://localhost:10044

**验证问题**:
```bash
# 在容器内检查设备（会返回空）
docker exec smart-cat smartctl --scan-open

# 在 macOS 上检查设备（能看到 diskX）
smartctl --scan-open
```

### 4. 权限错误

**问题**: `Permission denied` 或 `Operation not permitted`

**解决**: 使用 `sudo` 运行程序（Linux/macOS），或以管理员身份运行（Windows）

### 5. smartctl not found

**问题**: `smartctl not found`

**解决**: 安装 smartmontools（见"前置要求"部分）

### 6. USB 外接硬盘显示错误

**问题**: USB 硬盘显示"无法读取 SMART 数据（可能是不支持的 USB 桥接芯片）"

**原因**: 部分 USB 转 SATA 桥接芯片不支持 SMART 数据透传

**解决方案**:
- **v2 版本已自动尝试多种 USB 桥接类型** (`sat`, `usbsunplus`, `usbjmicron`, `usbcypress`)
- 如果仍然无法读取，说明该 USB 芯片确实不支持 SMART 透传
- 建议：
  1. 更换支持 SMART 透传的 USB 硬盘盒
  2. 将硬盘直接连接到 SATA 接口
  3. 检查硬盘盒说明书是否支持 SMART

**已知支持的 USB 芯片**: JMicron JMS578, ASMedia ASM1153E, Sunplus SPIF30x

### 7. 检测不到设备

**问题**: 页面显示"未检测到支持 SMART 的设备"

**可能原因**:
- 没有使用管理员权限运行
- 硬盘不支持 SMART（部分旧硬盘或虚拟硬盘）

### 8. 历史数据不显示

**问题**: 详情页看不到历史趋势图

**原因**: 程序首次运行时没有历史数据，需要等待至少 2 次采集（默认每小时一次）

**临时测试**: 修改 `main.go` 中的 `collectionTick` 为 `1 * time.Minute` 快速测试

## 配置

### 修改采集间隔

编辑 `main.go`:

```go
var (
    collectionTick = time.Hour  // 改为你想要的间隔，如 30 * time.Minute
)
```

### 修改服务器端口

编辑 `main.go`:

```go
addr := ":8080"  // 改为其他端口，如 ":3000"
```

### 清理旧数据

历史数据存储在 `./data/` 目录下，可以手动删除：

```bash
rm -rf ./data/*.csv
```

或者在代码中调用清理函数（保留最近 30 天）：

```go
storage.CleanOldRecords(30)
```

## 技术栈

- **后端**: Go (标准库 + smartctl)
- **前端**: 原生 HTML/CSS/JavaScript + Chart.js
- **存储**: CSV 文件（无数据库）

## 性能

- **内存占用**: < 20MB
- **CPU 占用**: 采集时短暂峰值，其余时间接近 0
- **存储开销**: 约 10KB/小时/10设备 ≈ 87MB/年

## 安全性

- ✅ 只读操作，不修改硬盘数据
- ✅ 本地运行，无网络请求
- ✅ 无身份认证（假设在可信环境运行）

**注意**: 默认绑定到 `0.0.0.0:8080`，如果在公网服务器运行，建议修改为 `127.0.0.1:8080` 或添加防火墙规则。

## 限制

1. **需要管理员权限** - 读取 SMART 数据需要 root/admin 权限
2. **依赖 smartctl** - 必须安装 smartmontools
3. **USB 硬盘支持** - 部分 USB 转接器不支持 SMART 穿透
4. **虚拟机支持** - 虚拟机中的虚拟硬盘通常不支持 SMART

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request。

遵循 Linus 的代码风格：
- 简洁优于复杂
- 消除特殊情况
- 数据结构第一
- 实用主义至上

---

**"好程序员担心数据结构，而不是代码。"** - Linus Torvalds
