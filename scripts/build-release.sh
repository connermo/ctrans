#!/bin/bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查是否提供版本号
if [ $# -eq 0 ]; then
    print_error "请提供版本号，例如: $0 v1.0.0"
    exit 1
fi

VERSION=$1
RELEASE_DIR="releases/${VERSION}"

print_info "开始构建 ctrans ${VERSION}..."

# 创建release目录
mkdir -p "${RELEASE_DIR}"

# 设置编译参数
LDFLAGS="-X main.version=${VERSION} -s -w"

print_info "设置编译参数: ${LDFLAGS}"

# 构建函数
build_binary() {
    local os=$1
    local arch=$2
    local component=$3
    local output_name=$4
    
    print_info "构建 ${component} for ${os}/${arch}..."
    
    GOOS=${os} GOARCH=${arch} go build \
        -ldflags="${LDFLAGS}" \
        -o "${RELEASE_DIR}/${output_name}" \
        ./${component}
    
    if [ $? -eq 0 ]; then
        print_success "✓ ${output_name}"
    else
        print_error "✗ 构建 ${output_name} 失败"
        exit 1
    fi
}

print_info "构建服务器端二进制文件..."

# 服务器端二进制文件
build_binary "linux" "amd64" "server" "ctrans-server-linux-amd64"
build_binary "linux" "arm64" "server" "ctrans-server-linux-arm64"
build_binary "darwin" "amd64" "server" "ctrans-server-macos-amd64"
build_binary "darwin" "arm64" "server" "ctrans-server-macos-arm64"
build_binary "windows" "amd64" "server" "ctrans-server-windows-amd64.exe"

print_info "构建客户端二进制文件..."

# 客户端二进制文件
build_binary "linux" "amd64" "client" "ctrans-linux-amd64"
build_binary "linux" "arm64" "client" "ctrans-linux-arm64"
build_binary "darwin" "amd64" "client" "ctrans-macos-amd64"
build_binary "darwin" "arm64" "client" "ctrans-macos-arm64"
build_binary "windows" "amd64" "client" "ctrans-windows-amd64.exe"

print_info "生成校验和文件..."

# 生成校验和
cd "${RELEASE_DIR}"
sha256sum * > checksums.txt
cd - > /dev/null

print_success "校验和文件已生成: ${RELEASE_DIR}/checksums.txt"

# 显示文件信息
print_info "构建完成的文件:"
ls -lh "${RELEASE_DIR}"

# 生成使用说明
cat > "${RELEASE_DIR}/README.txt" << EOF
ctrans ${VERSION} - 跨平台文件传输工具

📦 二进制文件说明:

服务器端:
- ctrans-server-linux-amd64     : Linux x86-64
- ctrans-server-linux-arm64     : Linux ARM64  
- ctrans-server-macos-amd64     : macOS x86-64
- ctrans-server-macos-arm64     : macOS ARM64 (Apple Silicon)
- ctrans-server-windows-amd64.exe : Windows x86-64

客户端:
- ctrans-linux-amd64            : Linux x86-64
- ctrans-linux-arm64            : Linux ARM64
- ctrans-macos-amd64            : macOS x86-64
- ctrans-macos-arm64            : macOS ARM64 (Apple Silicon)
- ctrans-windows-amd64.exe      : Windows x86-64

🚀 快速开始:

1. 启动服务器:
   Linux/macOS: ./ctrans-server-linux-amd64 -port 9000 -key "your-key"
   Windows:     ctrans-server-windows-amd64.exe -port 9000 -key "your-key"

2. 使用客户端:
   上传: ./ctrans-linux-amd64 file.txt server:9000
   下载: ./ctrans-linux-amd64 server:9000/file.txt
   列表: ./ctrans-linux-amd64 server:9000

3. 网页界面:
   访问 http://server:9000

🔐 安全验证:
   sha256sum -c checksums.txt

详细文档: https://github.com/your-username/ctrans
EOF

print_success "✅ 构建完成！"
print_info "📁 文件位置: ${RELEASE_DIR}"
print_info "🔗 可以手动上传到 GitHub Releases 或推送 tag 触发自动发布"

# 提示如何创建tag和推送
print_info ""
print_info "💡 要触发自动发布，请运行:"
print_warning "git tag ${VERSION}"
print_warning "git push origin ${VERSION}" 