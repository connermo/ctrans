# ctrans 预编译版本

这个目录包含了不同平台的ctrans预编译执行文件。

## 可用版本

### Linux x86-64
- **服务器**: `ctrans-server-linux-amd64`
- **客户端**: `ctrans-linux-amd64`

### macOS x86-64 (Intel Mac)
- **服务器**: `ctrans-server-macos-amd64`
- **客户端**: `ctrans-macos-amd64`

### macOS ARM64 (Apple Silicon)
- **服务器**: `ctrans-server-macos-arm64`
- **客户端**: `ctrans-macos-arm64`

## 安装

1. 根据你的系统选择对应的版本
2. 下载服务器和客户端文件
3. 添加执行权限：
   ```bash
   chmod +x ctrans-server-* ctrans-*
   ```

## 使用示例

**启动服务器：**
```bash
# Linux
./ctrans-server-linux-amd64 -port 9000

# macOS Intel
./ctrans-server-macos-amd64 -port 9000

# macOS Apple Silicon
./ctrans-server-macos-arm64 -port 9000
```

**使用客户端：**
```bash
# Linux
./ctrans-linux-amd64 myfile.txt server:9000

# macOS Intel
./ctrans-macos-amd64 myfile.txt server:9000

# macOS Apple Silicon
./ctrans-macos-arm64 myfile.txt server:9000
```

## 重命名（可选）

为了方便使用，你可以将文件重命名：
```bash
# 服务器
mv ctrans-server-linux-amd64 ctrans-server
mv ctrans-server-macos-amd64 ctrans-server
mv ctrans-server-macos-arm64 ctrans-server

# 客户端
mv ctrans-linux-amd64 ctrans
mv ctrans-macos-amd64 ctrans
mv ctrans-macos-arm64 ctrans
```

更多使用说明请参考主目录的 README.md 文件。 