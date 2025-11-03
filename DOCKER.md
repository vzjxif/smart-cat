# Docker 部署指南

## ⚠️ 平台限制

| 平台 | Docker 支持 | 说明 |
|------|-----------|------|
| **Linux** | ✅ 完美支持 | 推荐使用 Docker |
| **macOS** | ❌ 不支持 | Docker Desktop 无法访问物理硬盘 |
| **Windows** | ⚠️ 部分支持 | WSL2 下可能工作，但建议本地部署 |

### macOS 用户必读

**为什么不支持？**

Docker Desktop for Mac 的架构：
```
macOS 宿主机
    ↓
Linux 虚拟机 (Docker Desktop VM)
    ↓
Docker 容器
```

**问题**：
- 容器运行在 Linux VM 中，看不到 macOS 的 `/dev/diskX` 设备
- `--privileged` 和 `--device` 只能访问虚拟机的虚拟磁盘
- 这是 Docker Desktop 的设计限制，无法绕过

**解决方案**：
```bash
# macOS 请直接运行
brew install smartmontools
go build
sudo ./smart-cat
```

---

## 快速开始（仅 Linux）

### 方式一：使用 docker-compose（推荐）

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 访问
open http://localhost:10044
```

### 方式二：使用 docker run

```bash
# 构建镜像
docker build -t smart-cat:latest .

# 运行（privileged 模式）
docker run -d \
  --name smart-cat \
  --privileged \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v /sys:/sys:ro \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  smart-cat:latest

# 运行（只挂载特定设备，更安全）
docker run -d \
  --name smart-cat \
  --device=/dev/sda \
  --device=/dev/sdb \
  --device=/dev/sdc \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v /sys:/sys:ro \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  smart-cat:latest
```

## 镜像特性

- **超小体积**: 25.5 MB（多阶段构建）
- **自包含**: 内置 smartmontools
- **跨平台**: 支持 linux/amd64, linux/arm64
- **健康检查**: 自动监控容器状态
- **资源限制**: 默认限制 CPU 0.5核，内存 128MB

## 目录说明

```
smart-cat/
├── Dockerfile          # 镜像构建文件
├── docker-compose.yml  # 编排配置
├── .dockerignore       # 排除文件
└── data/              # 持久化数据（自动创建）
    └── *.csv          # 历史记录
```

## 配置选项

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `TZ` | 时区 | `UTC` |

### 端口

| 端口 | 说明 |
|------|------|
| `8080` | Web 界面 |

### 挂载点

| 容器路径 | 说明 | 必需 |
|----------|------|------|
| `/app/data` | 历史数据目录 | ✅ 推荐 |
| `/sys` | 系统设备信息 | ✅ 必需 |

### 设备权限

**方式一：Privileged 模式**
```yaml
privileged: true
```
- ✅ 简单，自动访问所有设备
- ❌ 安全性较低

**方式二：指定设备（推荐）**
```yaml
devices:
  - /dev/sda:/dev/sda
  - /dev/sdb:/dev/sdb
```
- ✅ 安全，仅暴露必要设备
- ❌ 需要手动列出所有硬盘

## 常用命令

```bash
# 启动
docker-compose up -d

# 停止
docker-compose down

# 重启
docker-compose restart

# 查看日志
docker-compose logs -f

# 查看实时日志（最近100行）
docker-compose logs --tail=100 -f

# 进入容器
docker exec -it smart-cat sh

# 手动采集数据
docker exec smart-cat smartctl --scan

# 查看容器资源使用
docker stats smart-cat

# 更新镜像
docker-compose pull
docker-compose up -d

# 清理旧镜像
docker image prune -f
```

## 故障排查

### 1. 容器启动失败

```bash
# 查看日志
docker logs smart-cat

# 检查设备权限
docker exec smart-cat smartctl --scan
```

### 2. 无法访问硬盘

**问题**: `Permission denied` 或检测不到设备

**解决**:
```bash
# 方案一：确保使用 privileged 模式
docker run --privileged ...

# 方案二：检查设备挂载
ls -la /dev/sd*
docker run --device=/dev/sda --device=/dev/sdb ...
```

### 3. 历史数据丢失

**问题**: 容器重启后数据消失

**解决**: 确保挂载了数据目录
```bash
docker run -v $(pwd)/data:/app/data ...
```

### 4. 时区不正确

**解决**: 设置 TZ 环境变量
```bash
docker run -e TZ=Asia/Shanghai ...
```

## 性能优化

### 资源限制

```yaml
deploy:
  resources:
    limits:
      cpus: '0.5'      # 限制 CPU 使用
      memory: 128M     # 限制内存
    reservations:
      cpus: '0.1'
      memory: 32M
```

### 采集间隔

修改源码 `main.go`:
```go
var (
    collectionTick = 30 * time.Minute  // 改为30分钟采集一次
)
```

然后重新构建镜像。

## 网络配置

### 使用自定义端口

```bash
docker run -p 3000:8080 smart-cat:latest
```

### 仅限本地访问

```bash
docker run -p 127.0.0.1:8080:8080 smart-cat:latest
```

### 使用 nginx 反向代理

```nginx
server {
    listen 80;
    server_name smart.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 安全建议

1. **不要在公网暴露** - 本工具无身份认证
2. **使用设备挂载代替 privileged** - 最小权限原则
3. **定期备份数据目录** - 防止数据丢失
4. **使用反向代理 + HTTPS** - 如需远程访问

## 多架构支持

构建多架构镜像：

```bash
# 安装 buildx
docker buildx create --use

# 构建多架构镜像
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/smart-cat:latest \
  --push .
```

## 与宿主机工具对比

| 特性 | Docker | 宿主机运行 |
|------|--------|-----------|
| 环境配置 | ✅ 一键启动 | ❌ 需手动安装 |
| 权限要求 | ✅ 容器内 root | ❌ 需 sudo |
| 资源隔离 | ✅ 限制资源 | ❌ 无隔离 |
| 升级更新 | ✅ 重建镜像 | ❌ 重新编译 |
| 卸载清理 | ✅ 删除容器 | ❌ 手动清理 |

---

**推荐使用 Docker 部署** - 最简单、最干净、最易维护。
