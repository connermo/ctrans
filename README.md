# ctrans

一个基于 HTTP 的文件传输工具，专为控制台环境设计。支持大文件传输、断点续传、并发上传等功能。

## 特性

- 基于 HTTP 协议，无需 SSH 访问
- 支持任意大小的文件传输（无固定大小限制）
- 智能磁盘空间管理
- 支持自定义服务器监听地址和端口
- 支持服务密钥认证
- 分片上传（每片 10MB）
- 断点续传
- 并发上传
- 文件完整性校验
- 自动恢复中断的上传
- 支持列出服务器上的文件
- 支持下载文件
- **类似scp的简洁命令格式**
- **现代化网页界面**
- **目录上传下载支持**

## 安装

### 下载预编译版本（推荐）

前往 [Releases 页面](https://github.com/your-username/ctrans/releases) 下载适合你系统的预编译版本：

- **Linux x86-64**: `ctrans-linux-amd64`, `ctrans-server-linux-amd64`
- **macOS x86-64**: `ctrans-macos-amd64`, `ctrans-server-macos-amd64`  
- **macOS ARM64**: `ctrans-macos-arm64`, `ctrans-server-macos-arm64`
- **Windows x86-64**: `ctrans-windows-amd64.exe`, `ctrans-server-windows-amd64.exe`

### 从源码编译

如果需要从源码编译，请参考 [RELEASE.md](./RELEASE.md)。

## 使用方法

### 启动服务器

```bash
./ctrans-server [-host HOST] [-port PORT] [-key SERVICE_KEY]
```

参数说明：
- `-host`: 服务器监听地址（可选，默认为本地主机名）
- `-port`: 服务器监听端口（可选，默认为 9000）
- `-key`: 服务密钥（可选，用于认证）

示例：
```bash
# 使用默认设置启动服务器
./ctrans-server

# 指定监听地址和端口
./ctrans-server -host 192.168.1.100 -port 8080

# 使用服务密钥启动服务器
./ctrans-server -key "your-secret-key"
```

### 使用客户端

#### 基本命令格式（类似scp）

**列出服务器文件：**
```bash
./ctrans <server:port>
```

**上传文件：**
```bash
./ctrans <local-file> <server:port>
```

**下载文件：**
```bash
./ctrans <server:port>/<filename> [local-path]
```

#### 示例

```bash
# 列出服务器上的文件
./ctrans localhost:9000

# 上传文件
./ctrans myfile.txt localhost:9000
./ctrans /path/to/large-file.tar localhost:9000

# 下载文件
./ctrans localhost:9000/myfile.txt                    # 下载到当前目录
./ctrans localhost:9000/myfile.txt ./downloads/       # 下载到指定目录

# 使用服务密钥
./ctrans -key "your-secret-key" myfile.txt server:9000
./ctrans -key "your-secret-key" server:9000/myfile.txt
```

#### 高级选项

```bash
# 恢复上传
./ctrans -resume <file-id> <server:port>

# 使用服务密钥
./ctrans -key "your-secret-key" <command>

# 显示帮助
./ctrans --help
```

### 网页界面

访问 `http://server:port` 使用现代化的网页界面，支持：

- 🔐 **密钥认证**: 安全的服务密钥验证
- 📁 **目录上传**: 支持完整目录结构上传
- 🎯 **拖拽上传**: 直观的拖拽上传体验
- 📊 **实时进度**: 上传进度实时显示
- 📋 **文件管理**: 浏览和下载服务器文件
- 🍪 **自动登录**: 密钥自动保存和验证

## 技术细节

### 上传过程
1. 客户端初始化上传，获取文件 ID
2. 服务器检查可用磁盘空间
3. 文件被分成 10MB 的块
4. 客户端并发上传文件块
5. 每个块上传后，服务器验证其完整性
6. 如果上传中断，客户端会自动检测并恢复
7. 所有块上传完成后，服务器验证整个文件的完整性

### 下载过程
1. 客户端请求文件信息
2. 如果下载中断，支持从断点处继续下载
3. 下载完成后验证文件完整性

### 安全特性
- 服务密钥认证：所有请求都需要提供有效的服务密钥
- 文件完整性校验：使用 SHA-256 确保文件完整性
- 智能磁盘空间管理：服务器会在上传前检查可用空间

## 注意事项

1. 服务器会自动检查可用磁盘空间，确保有足够空间存储上传的文件
2. 建议在生产环境中使用服务密钥认证
3. 对于大文件传输，建议使用稳定的网络连接
4. 上传状态文件保存在客户端的 `.upload_state` 目录中
5. 服务器默认监听端口为 9000，可以通过 `-port` 参数修改

## 大文件传输建议

1. 确保服务器有足够的磁盘空间
2. 使用稳定的网络连接
3. 如果传输中断，客户端会自动恢复上传
4. 建议在传输大文件时使用服务密钥认证
5. 监控服务器磁盘空间使用情况

## 发布和开发

- 发布流程和编译说明请参考 [RELEASE.md](./RELEASE.md)
- 项目采用自动化发布流程，支持多平台预编译版本

## 许可证

本项目采用 [MIT 许可证](./LICENSE)。 