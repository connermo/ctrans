# ctrans 发布指南

本项目提供了多种发布方式，支持自动化构建和发布流程。

## 🚀 发布方式

### 1. 自动发布（推荐）

**触发条件**：推送以 `v` 开头的 tag

```bash
# 创建并推送 tag
git tag v1.0.0
git push origin v1.0.0
```

**流程**：
1. GitHub Actions 自动检测到 tag
2. 构建所有平台的二进制文件
3. 生成校验和文件
4. 创建 GitHub Release
5. 上传所有文件到 Release

### 2. 手动发布

在 GitHub 网页上：
1. 进入 **Actions** 标签页
2. 选择 **Manual Release** workflow
3. 点击 **Run workflow**
4. 输入版本号（如 `v1.0.1`）
5. 选择是否为预发布版本
6. 点击 **Run workflow** 按钮

### 3. 本地构建

使用提供的脚本在本地构建：

```bash
# 给脚本执行权限（首次运行）
chmod +x scripts/build-release.sh

# 构建指定版本
./scripts/build-release.sh v1.0.0
```

构建完成后，文件会保存在 `releases/v1.0.0/` 目录中。

## 📦 构建产物

每次发布会生成以下文件：

### 服务器端
- `ctrans-server-linux-amd64` - Linux x86-64
- `ctrans-server-linux-arm64` - Linux ARM64
- `ctrans-server-macos-amd64` - macOS x86-64
- `ctrans-server-macos-arm64` - macOS ARM64 (Apple Silicon)
- `ctrans-server-windows-amd64.exe` - Windows x86-64

### 客户端
- `ctrans-linux-amd64` - Linux x86-64
- `ctrans-linux-arm64` - Linux ARM64
- `ctrans-macos-amd64` - macOS x86-64
- `ctrans-macos-arm64` - macOS ARM64 (Apple Silicon)
- `ctrans-windows-amd64.exe` - Windows x86-64

### 其他文件
- `checksums.txt` - SHA256 校验和文件
- `README.txt` - 使用说明（本地构建时生成）

## 🔐 安全验证

所有发布都包含 SHA256 校验和，用于验证文件完整性：

```bash
# 下载 checksums.txt 后验证
sha256sum -c checksums.txt
```

## 📋 版本命名规范

- **正式版本**：`v1.0.0`、`v1.2.3`
- **预发布版本**：`v1.0.0-beta.1`、`v1.0.0-rc.1`
- **开发版本**：`v1.0.0-dev.20231201`

## 🛠️ 开发流程

### 准备发布

1. **更新版本信息**
   ```bash
   # 更新 README.md 中的版本说明
   # 更新 CHANGELOG.md（如果有）
   ```

2. **本地测试**
   ```bash
   # 测试本地构建
   ./scripts/build-release.sh v1.0.0-test
   
   # 测试二进制文件
   ./releases/v1.0.0-test/ctrans-server-linux-amd64 --version
   ./releases/v1.0.0-test/ctrans-linux-amd64 --version
   ```

3. **提交代码**
   ```bash
   git add .
   git commit -m "chore: prepare release v1.0.0"
   git push origin main
   ```

### 发布版本

1. **创建 tag**
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **监控构建**
   - 在 GitHub Actions 中查看构建状态
   - 确认所有平台构建成功

3. **验证发布**
   - 检查 GitHub Releases 页面
   - 测试下载的二进制文件
   - 验证校验和

## 🔧 故障排除

### 构建失败

如果自动构建失败：

1. **检查日志**
   - 在 GitHub Actions 中查看详细日志
   - 识别失败的具体步骤

2. **本地复现**
   ```bash
   # 使用相同的版本号本地构建
   ./scripts/build-release.sh v1.0.0
   ```

3. **修复问题**
   - 修复代码问题
   - 重新提交并推送
   - 删除旧 tag 并重新创建

   ```bash
   # 删除本地和远程 tag
   git tag -d v1.0.0
   git push origin :refs/tags/v1.0.0
   
   # 重新创建 tag
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

### 手动修复发布

如果需要手动修复发布：

1. **本地构建**
   ```bash
   ./scripts/build-release.sh v1.0.0
   ```

2. **手动上传**
   - 在 GitHub Releases 页面编辑发布
   - 手动上传修复的文件
   - 更新发布说明

## 🌟 最佳实践

1. **版本号递增**：确保每次发布都使用递增的版本号
2. **测试验证**：发布前在本地充分测试
3. **更新文档**：及时更新 README 和相关文档
4. **备份重要版本**：保留重要版本的源码备份
5. **安全检查**：定期检查依赖项安全性

## 📞 联系支持

如果在发布过程中遇到问题，请：

1. 检查此文档的故障排除部分
2. 查看 GitHub Issues 中的类似问题
3. 创建新的 Issue 描述问题 