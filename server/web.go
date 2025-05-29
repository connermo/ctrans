package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// handleWebUpload 提供网页上传界面
func handleWebUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	needsAuth := serviceKey != ""

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ctrans - 文件上传</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #333;
        }
        
        .container {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 20px 40px rgba(0,0,0,0.1);
            max-width: 500px;
            width: 90%%;
        }
        
        .header {
            text-align: center;
            margin-bottom: 2rem;
        }
        
        .header h1 {
            color: #333;
            font-size: 2rem;
            margin-bottom: 0.5rem;
        }
        
        .header p {
            color: #666;
            font-size: 1rem;
        }
        
        /* 登录界面样式 */
        .login-section {
            text-align: center;
        }
        
        .login-title {
            color: #333;
            font-size: 1.5rem;
            margin-bottom: 1rem;
        }
        
        .login-input {
            width: 100%%;
            padding: 1rem;
            border: 2px solid #ddd;
            border-radius: 8px;
            margin-bottom: 1rem;
            font-size: 1rem;
            text-align: center;
        }
        
        .login-input:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        
        .login-btn {
            width: 100%%;
            padding: 1rem;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 1rem;
            cursor: pointer;
            transition: transform 0.2s ease;
        }
        
        .login-btn:hover {
            transform: translateY(-2px);
        }
        
        .login-btn:active {
            transform: translateY(0);
        }
        
        .login-error {
            color: #dc3545;
            margin-top: 1rem;
            text-align: center;
            display: none;
        }
        
        /* 主界面样式 */
        .main-section {
            display: none;
        }
        
        .logout-btn {
            position: absolute;
            top: 1rem;
            right: 1rem;
            background: #f8f9fa;
            border: 1px solid #dee2e6;
            color: #6c757d;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9rem;
        }
        
        .logout-btn:hover {
            background: #e9ecef;
        }
        
        .upload-area {
            border: 2px dashed #ddd;
            border-radius: 8px;
            padding: 3rem 2rem;
            text-align: center;
            transition: all 0.3s ease;
            cursor: pointer;
            margin-bottom: 1rem;
        }
        
        .upload-area:hover,
        .upload-area.dragover {
            border-color: #667eea;
            background-color: #f8f9ff;
        }
        
        .upload-area.uploading {
            border-color: #28a745;
            background-color: #f8fff9;
        }
        
        .upload-icon {
            font-size: 3rem;
            color: #ddd;
            margin-bottom: 1rem;
        }
        
        .upload-text {
            color: #666;
            font-size: 1.1rem;
            margin-bottom: 0.5rem;
        }
        
        .upload-hint {
            color: #999;
            font-size: 0.9rem;
        }
        
        .file-input {
            display: none;
        }
        
        .progress-container {
            display: none;
            margin-top: 1rem;
        }
        
        .progress-bar {
            width: 100%%;
            height: 8px;
            background-color: #f0f0f0;
            border-radius: 4px;
            overflow: hidden;
        }
        
        .progress-fill {
            height: 100%%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            border-radius: 4px;
            transition: width 0.3s ease;
            width: 0%%;
        }
        
        .progress-text {
            text-align: center;
            margin-top: 0.5rem;
            color: #666;
            font-size: 0.9rem;
        }
        
        .result {
            margin-top: 1rem;
            padding: 1rem;
            border-radius: 6px;
            text-align: center;
            display: none;
        }
        
        .result.success {
            background-color: #d4edda;
            color: #155724;
            border: 1px solid #c3e6cb;
        }
        
        .result.error {
            background-color: #f8d7da;
            color: #721c24;
            border: 1px solid #f5c6cb;
        }
        
        .file-list {
            margin-top: 2rem;
            border-top: 1px solid #eee;
            padding-top: 1rem;
        }
        
        .file-list h3 {
            color: #333;
            margin-bottom: 1rem;
            font-size: 1.1rem;
        }
        
        .file-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.5rem 0;
            border-bottom: 1px solid #f0f0f0;
        }
        
        .file-item:last-child {
            border-bottom: none;
        }
        
        .file-name {
            color: #333;
            text-decoration: none;
        }
        
        .file-name:hover {
            color: #667eea;
        }
        
        .file-size {
            color: #666;
            font-size: 0.9rem;
        }
        
        .upload-options {
            display: flex;
            gap: 0.5rem;
            margin-bottom: 1rem;
            justify-content: center;
        }
        
        .upload-option-btn {
            padding: 0.75rem 1.5rem;
            border: 2px solid #ddd;
            background: white;
            border-radius: 8px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: all 0.3s ease;
        }
        
        .upload-option-btn:hover {
            border-color: #667eea;
            background-color: #f8f9ff;
        }
        
        .upload-option-btn.active {
            border-color: #667eea;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📁 ctrans</h1>
            <p>简单、快速的文件传输工具</p>
        </div>
        
        <!-- 登录界面 -->
        <div id="loginSection" class="login-section">
            <h2 class="login-title">🔐 请输入服务密钥</h2>
            <input type="password" id="loginKey" class="login-input" placeholder="输入密钥..." autocomplete="off">
            <button id="loginBtn" class="login-btn">登录</button>
            <div id="loginError" class="login-error">密钥错误，请重试</div>
        </div>
        
        <!-- 主界面 -->
        <div id="mainSection" class="main-section">
            <button id="logoutBtn" class="logout-btn">退出登录</button>
            
            <div class="upload-area" id="uploadArea">
                <div class="upload-icon">📤</div>
                <div class="upload-text">点击选择文件或拖拽文件到此处</div>
                <div class="upload-hint">支持任意格式文件和目录</div>
                <input type="file" id="fileInput" class="file-input" multiple>
                <input type="file" id="folderInput" class="file-input" webkitdirectory multiple>
            </div>
            
            <div class="upload-options">
                <button id="fileBtn" class="upload-option-btn active">📄 选择文件</button>
                <button id="folderBtn" class="upload-option-btn">📁 选择目录</button>
            </div>
            
            <div class="progress-container" id="progressContainer">
                <div class="progress-bar">
                    <div class="progress-fill" id="progressFill"></div>
                </div>
                <div class="progress-text" id="progressText">准备上传...</div>
            </div>
            
            <div class="result" id="result"></div>
            
            <div class="file-list" id="fileList">
                <h3>服务器文件</h3>
                <div id="files">加载中...</div>
            </div>
        </div>
    </div>

    <script>
        const needsAuth = %t;
        let currentServiceKey = '';
        
        // DOM 元素
        const loginSection = document.getElementById('loginSection');
        const mainSection = document.getElementById('mainSection');
        const loginKeyInput = document.getElementById('loginKey');
        const loginBtn = document.getElementById('loginBtn');
        const loginError = document.getElementById('loginError');
        const logoutBtn = document.getElementById('logoutBtn');
        const uploadArea = document.getElementById('uploadArea');
        const fileInput = document.getElementById('fileInput');
        const folderInput = document.getElementById('folderInput');
        const fileBtn = document.getElementById('fileBtn');
        const folderBtn = document.getElementById('folderBtn');
        const progressContainer = document.getElementById('progressContainer');
        const progressFill = document.getElementById('progressFill');
        const progressText = document.getElementById('progressText');
        const result = document.getElementById('result');
        
        let currentUploadMode = 'file'; // 'file' or 'folder'
        
        // Cookie 操作函数
        function setCookie(name, value, days) {
            const expires = new Date();
            expires.setTime(expires.getTime() + (days * 24 * 60 * 60 * 1000));
            document.cookie = name + '=' + value + ';expires=' + expires.toUTCString() + ';path=/';
        }
        
        function getCookie(name) {
            const nameEQ = name + '=';
            const ca = document.cookie.split(';');
            for (let i = 0; i < ca.length; i++) {
                let c = ca[i];
                while (c.charAt(0) === ' ') c = c.substring(1, c.length);
                if (c.indexOf(nameEQ) === 0) return c.substring(nameEQ.length, c.length);
            }
            return null;
        }
        
        function deleteCookie(name) {
            document.cookie = name + '=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
        }
        
        // 验证密钥
        async function validateKey(key) {
            try {
                const headers = needsAuth ? {'X-Service-Key': key} : {};
                const response = await fetch('/files', { headers });
                return response.ok;
            } catch (error) {
                return false;
            }
        }
        
        // 显示主界面
        function showMainInterface() {
            loginSection.style.display = 'none';
            mainSection.style.display = 'block';
            loadFiles();
        }
        
        // 显示登录界面
        function showLoginInterface() {
            loginSection.style.display = 'block';
            mainSection.style.display = 'none';
            loginError.style.display = 'none';
            loginKeyInput.value = '';
            currentServiceKey = '';
        }
        
        // 登录处理
        async function handleLogin() {
            const key = loginKeyInput.value.trim();
            if (!key) {
                loginError.textContent = '请输入密钥';
                loginError.style.display = 'block';
                return;
            }
            
            loginBtn.textContent = '验证中...';
            loginBtn.disabled = true;
            
            const isValid = await validateKey(key);
            
            if (isValid) {
                currentServiceKey = key;
                setCookie('service_key', key, 7); // 保存7天
                showMainInterface();
            } else {
                loginError.textContent = '密钥错误，请重试';
                loginError.style.display = 'block';
            }
            
            loginBtn.textContent = '登录';
            loginBtn.disabled = false;
        }
        
        // 登出处理
        function handleLogout() {
            deleteCookie('service_key');
            showLoginInterface();
        }
        
        // 初始化
        async function init() {
            if (!needsAuth) {
                showMainInterface();
                return;
            }
            
            const savedKey = getCookie('service_key');
            if (savedKey) {
                const isValid = await validateKey(savedKey);
                if (isValid) {
                    currentServiceKey = savedKey;
                    showMainInterface();
                    return;
                }
                deleteCookie('service_key');
            }
            
            showLoginInterface();
        }
        
        // 事件监听器
        loginBtn.addEventListener('click', handleLogin);
        loginKeyInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                handleLogin();
            }
        });
        logoutBtn.addEventListener('click', handleLogout);
        
        // 上传模式切换
        fileBtn.addEventListener('click', () => {
            currentUploadMode = 'file';
            fileBtn.classList.add('active');
            folderBtn.classList.remove('active');
            document.querySelector('.upload-text').textContent = '点击选择文件或拖拽文件到此处';
        });
        
        folderBtn.addEventListener('click', () => {
            currentUploadMode = 'folder';
            folderBtn.classList.add('active');
            fileBtn.classList.remove('active');
            document.querySelector('.upload-text').textContent = '点击选择目录或拖拽目录到此处';
        });
        
        // 文件上传相关事件
        uploadArea.addEventListener('click', () => {
            if (currentUploadMode === 'file') {
                fileInput.click();
            } else {
                folderInput.click();
            }
        });
        
        uploadArea.addEventListener('dragover', (e) => {
            e.preventDefault();
            uploadArea.classList.add('dragover');
        });
        
        uploadArea.addEventListener('dragleave', () => {
            uploadArea.classList.remove('dragover');
        });
        
        uploadArea.addEventListener('drop', (e) => {
            e.preventDefault();
            uploadArea.classList.remove('dragover');
            
            const items = e.dataTransfer.items;
            const files = [];
            
            // 处理拖拽的文件和目录
            if (items) {
                for (let i = 0; i < items.length; i++) {
                    const item = items[i];
                    if (item.kind === 'file') {
                        const entry = item.webkitGetAsEntry();
                        if (entry) {
                            if (entry.isDirectory) {
                                // 递归读取目录
                                readDirectory(entry, files).then(() => {
                                    if (files.length > 0) {
                                        uploadFiles(files);
                                    }
                                });
                                return;
                            } else {
                                files.push(item.getAsFile());
                            }
                        }
                    }
                }
                
                if (files.length > 0) {
                    uploadFiles(files);
                }
            } else {
                // 兼容旧浏览器
                const draggedFiles = Array.from(e.dataTransfer.files);
                if (draggedFiles.length > 0) {
                    uploadFiles(draggedFiles);
                }
            }
        });
        
        // 递归读取目录内容
        function readDirectory(dirEntry, fileList) {
            return new Promise((resolve) => {
                const dirReader = dirEntry.createReader();
                const readEntries = () => {
                    dirReader.readEntries((entries) => {
                        if (entries.length === 0) {
                            resolve();
                            return;
                        }
                        
                        const promises = entries.map(entry => {
                            if (entry.isDirectory) {
                                return readDirectory(entry, fileList);
                            } else {
                                return new Promise((fileResolve) => {
                                    entry.file(file => {
                                        // 保持目录结构信息
                                        file.webkitRelativePath = entry.fullPath.substring(1);
                                        fileList.push(file);
                                        fileResolve();
                                    });
                                });
                            }
                        });
                        
                        Promise.all(promises).then(() => {
                            readEntries(); // 继续读取更多条目
                        });
                    });
                };
                readEntries();
            });
        }
        
        // 上传文件
        async function uploadFiles(files) {
            if (files.length === 0) return;
            
            uploadArea.classList.add('uploading');
            progressContainer.style.display = 'block';
            result.style.display = 'none';
            
            try {
                for (let i = 0; i < files.length; i++) {
                    const file = files[i];
                    progressText.textContent = '上传中 ' + (i + 1) + '/' + files.length + ': ' + file.name;
                    
                    await uploadFile(file);
                    
                    const progress = ((i + 1) / files.length) * 100;
                    progressFill.style.width = progress + '%%';
                }
                
                showResult('success', '上传成功！共上传 ' + files.length + ' 个文件。');
                loadFiles(); // 刷新文件列表
                
            } catch (error) {
                showResult('error', '上传失败：' + error.message);
            } finally {
                uploadArea.classList.remove('uploading');
                progressContainer.style.display = 'none';
                fileInput.value = '';
                folderInput.value = '';
            }
        }
        
        // 上传单个文件
        function uploadFile(file) {
            return new Promise((resolve, reject) => {
                const formData = new FormData();
                formData.append('file', file);
                
                // 如果有相对路径信息，发送给服务器
                if (file.webkitRelativePath) {
                    formData.append('relativePath', file.webkitRelativePath);
                }
                
                const xhr = new XMLHttpRequest();
                
                xhr.upload.addEventListener('progress', (e) => {
                    if (e.lengthComputable) {
                        const fileProgress = (e.loaded / e.total) * 100;
                        const displayPath = file.webkitRelativePath || file.name;
                        progressText.textContent = '上传中: ' + displayPath + ' (' + fileProgress.toFixed(1) + '%%)';
                    }
                });
                
                xhr.addEventListener('load', () => {
                    if (xhr.status === 200) {
                        resolve();
                    } else {
                        reject(new Error(xhr.responseText || '上传失败'));
                    }
                });
                
                xhr.addEventListener('error', () => {
                    reject(new Error('网络错误'));
                });
                
                xhr.open('POST', '/web-upload');
                
                // 添加认证头（必须在open之后）
                if (needsAuth && currentServiceKey) {
                    xhr.setRequestHeader('X-Service-Key', currentServiceKey);
                }
                
                xhr.send(formData);
            });
        }
        
        // 显示结果
        function showResult(type, message) {
            result.className = 'result ' + type;
            result.textContent = message;
            result.style.display = 'block';
        }
        
        // 加载文件列表
        async function loadFiles() {
            try {
                const headers = {};
                if (needsAuth && currentServiceKey) {
                    headers['X-Service-Key'] = currentServiceKey;
                }
                
                const response = await fetch('/files', { headers });
                if (!response.ok) {
                    throw new Error('无法获取文件列表');
                }
                
                const files = await response.json();
                displayFiles(files);
            } catch (error) {
                document.getElementById('files').textContent = '无法加载文件列表';
            }
        }
        
        // 显示文件列表
        function displayFiles(files) {
            const filesContainer = document.getElementById('files');
            
            if (!files || files.length === 0) {
                filesContainer.textContent = '暂无文件';
                return;
            }
            
            // 排序：目录在前，文件在后，按路径排序
            files.sort((a, b) => {
                if (a.is_dir !== b.is_dir) {
                    return a.is_dir ? -1 : 1;
                }
                return a.path.localeCompare(b.path);
            });
            
            let html = '';
            for (let i = 0; i < files.length; i++) {
                const file = files[i];
                const depth = (file.path.match(/\//g) || []).length;
                const indent = '&nbsp;'.repeat(depth * 4);
                const icon = file.is_dir ? '📁' : '📄';
                
                html += '<div class="file-item" style="padding-left: ' + (depth * 20) + 'px;">';
                
                if (file.is_dir) {
                    html += '<span class="file-name">' + indent + icon + ' ' + file.name + '/</span>';
                    html += '<span class="file-size">目录</span>';
                } else {
                    html += '<a href="/download/' + encodeURIComponent(file.path) + '" class="file-name" target="_blank">';
                    html += indent + icon + ' ' + file.name;
                    html += '</a>';
                    html += '<span class="file-size">' + formatFileSize(file.size) + '</span>';
                }
                
                html += '</div>';
            }
            filesContainer.innerHTML = html;
        }
        
        // 格式化文件大小
        function formatFileSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }
        
        // 页面加载时初始化
        init();
        
        fileInput.addEventListener('change', (e) => {
            const files = Array.from(e.target.files);
            if (files.length > 0) {
                uploadFiles(files);
            }
        });
        
        folderInput.addEventListener('change', (e) => {
            const files = Array.from(e.target.files);
            if (files.length > 0) {
                uploadFiles(files);
            }
        });
    </script>
</body>
</html>`,
		needsAuth)
}

// handleWebUploadFile 处理网页文件上传
func handleWebUploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 检查磁盘空间
	if err := checkDiskSpace(header.Size); err != nil {
		http.Error(w, fmt.Sprintf("Insufficient disk space: %v", err), http.StatusInsufficientStorage)
		return
	}

	// 获取相对路径（如果有的话）
	relativePath := r.FormValue("relativePath")
	var finalPath string

	if relativePath != "" {
		// 有相对路径，保持目录结构
		finalPath = filepath.Join(uploadDir, relativePath)

		// 创建必要的目录
		dir := filepath.Dir(finalPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}
	} else {
		// 没有相对路径，直接放在上传目录
		finalPath = filepath.Join(uploadDir, header.Filename)
	}

	// 创建目标文件
	finalFile, err := os.Create(finalPath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer finalFile.Close()

	// 复制文件并计算校验和
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(finalFile, hash), file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	checksum := hex.EncodeToString(hash.Sum(nil))

	// 返回成功响应
	response := map[string]interface{}{
		"status":   "success",
		"filename": header.Filename,
		"path":     relativePath,
		"size":     header.Size,
		"checksum": checksum,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
