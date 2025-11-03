#!/bin/bash
# SMART Cat 快速启动脚本

set -e

echo "🐱 SMART Cat - 硬盘健康监控工具"
echo "=================================="
echo ""

# 检查 smartctl 是否安装
if ! command -v smartctl &> /dev/null; then
    echo "❌ 错误: 未检测到 smartctl"
    echo ""
    echo "请先安装 smartmontools:"
    echo "  macOS:   brew install smartmontools"
    echo "  Linux:   sudo apt install smartmontools"
    echo ""
    exit 1
fi

# 检查是否有 root 权限
if [ "$EUID" -ne 0 ]; then
    echo "⚠️  警告: 需要 root 权限读取 SMART 数据"
    echo ""
    echo "请使用 sudo 运行:"
    echo "  sudo ./run.sh"
    echo ""
    exit 1
fi

# 编译（如果需要）
if [ ! -f "./smart-cat" ]; then
    echo "📦 正在编译..."
    go build -o smart-cat
    echo "✅ 编译完成"
    echo ""
fi

# 启动服务器
echo "🚀 启动服务器..."
echo ""
echo "访问地址: http://localhost:8080"
echo "按 Ctrl+C 停止服务器"
echo ""

./smart-cat
