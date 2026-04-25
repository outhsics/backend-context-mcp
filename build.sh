#!/bin/bash
# 编译脚本 — 在 Mac 上运行，生成后端同事直接使用的二进制文件

set -e

echo "🔧 编译 byjyedu-backend-context..."

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ 未安装 Go，正在安装..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install go
    else
        echo "请先安装 Go: https://go.dev/dl/"
        exit 1
    fi
fi

echo "Go 版本: $(go version)"

# 下载依赖
echo "📦 下载依赖..."
go mod tidy

# 编译 macOS ARM64（后端同事 MacBook Air M4）
echo "🎯 编译 macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -o byjyedu-backend-context .

echo "✅ 编译完成！"
echo ""
echo "文件: $(pwd)/byjyedu-backend-context"
echo "大小: $(du -h byjyedu-backend-context | cut -f1)"
echo ""
echo "发给后端同事后，他运行："
echo "  chmod +x byjyedu-backend-context"
echo "  ./byjyedu-backend-context --dir /path/to/byjyedu-java"
