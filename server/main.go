package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	uploadDir    = "./uploads"
	tempDir      = "./temp"
	chunkSize    = 10 * 1024 * 1024   // 10MB per chunk
	minDiskSpace = 1024 * 1024 * 1024 // 1GB 最小剩余空间
	authHeader   = "X-Service-Key"    // 认证头
)

type UploadStatus struct {
	FileID      string    `json:"file_id"`
	FileName    string    `json:"file_name"`
	TotalSize   int64     `json:"total_size"`
	TotalChunks int       `json:"total_chunks"`
	ChunkSize   int64     `json:"chunk_size"`
	Uploaded    []int     `json:"uploaded_chunks"`
	StartTime   time.Time `json:"start_time"`
	LastUpdate  time.Time `json:"last_update"`
	Completed   bool      `json:"completed"`
	Checksum    string    `json:"checksum,omitempty"`
}

var (
	serviceKey     string // 服务密钥
	uploadStatuses = make(map[string]*UploadStatus)
	statusMutex    sync.RWMutex
)

// 中间件：验证服务密钥
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果未设置服务密钥，跳过验证
		if serviceKey == "" {
			next(w, r)
			return
		}

		// 获取请求头中的服务密钥
		key := r.Header.Get(authHeader)
		if key == "" {
			http.Error(w, "Service key required", http.StatusUnauthorized)
			return
		}

		// 验证服务密钥
		if key != serviceKey {
			http.Error(w, "Invalid service key", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func main() {
	// 定义命令行参数
	host := flag.String("host", "", "Server host address (default: all interfaces)")
	port := flag.String("port", "8080", "Server port number")
	key := flag.String("key", "", "Service key for authentication (optional)")
	flag.Parse()

	// 设置服务密钥
	serviceKey = *key

	// 创建必要的目录
	for _, dir := range []string{uploadDir, tempDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal("Failed to create directory:", err)
		}
	}

	// 设置路由（添加认证中间件）
	http.HandleFunc("/", handleWebUpload)                               // 网页上传界面
	http.HandleFunc("/web-upload", authMiddleware(handleWebUploadFile)) // 网页文件上传处理
	http.HandleFunc("/upload/init", authMiddleware(handleUploadInit))
	http.HandleFunc("/upload/chunk/", authMiddleware(handleChunkUpload))
	http.HandleFunc("/upload/status/", authMiddleware(handleUploadStatus))
	http.HandleFunc("/upload/complete/", authMiddleware(handleUploadComplete))
	http.HandleFunc("/download/", authMiddleware(handleDownload))
	http.HandleFunc("/files", authMiddleware(handleListFiles))

	// 构建服务器地址
	var serverAddr string
	if *host == "" {
		// 如果未指定主机地址，使用 localhost
		serverAddr = fmt.Sprintf(":%s", *port)
		fmt.Printf("Server started at http://localhost:%s\n", *port)
	} else {
		serverAddr = fmt.Sprintf("%s:%s", *host, *port)
		fmt.Printf("Server started at http://%s\n", serverAddr)
	}

	// 显示认证状态
	if serviceKey != "" {
		fmt.Println("Service key authentication enabled")
	} else {
		fmt.Println("Service key authentication disabled")
	}

	fmt.Println("Available endpoints:")
	fmt.Println("  - Init Upload:    POST http://localhost:" + *port + "/upload/init")
	fmt.Println("  - Upload Chunk:   POST http://localhost:" + *port + "/upload/chunk/<file_id>/<chunk_number>")
	fmt.Println("  - Upload Status:  GET  http://localhost:" + *port + "/upload/status/<file_id>")
	fmt.Println("  - Complete Upload: POST http://localhost:" + *port + "/upload/complete/<file_id>")
	fmt.Println("  - Download:       GET  http://localhost:" + *port + "/download/<filename>")
	fmt.Println("  - List Files:     GET  http://localhost:" + *port + "/files")

	log.Fatal(http.ListenAndServe(serverAddr, nil))
}

func handleUploadInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileName  string `json:"file_name"`
		TotalSize int64  `json:"total_size"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 检查磁盘空间
	if err := checkDiskSpace(req.TotalSize); err != nil {
		http.Error(w, fmt.Sprintf("Insufficient disk space: %v", err), http.StatusInsufficientStorage)
		return
	}

	// 生成文件ID
	fileID := generateFileID(req.FileName, req.TotalSize)
	totalChunks := int((req.TotalSize + chunkSize - 1) / chunkSize)

	status := &UploadStatus{
		FileID:      fileID,
		FileName:    req.FileName,
		TotalSize:   req.TotalSize,
		TotalChunks: totalChunks,
		ChunkSize:   chunkSize,
		Uploaded:    make([]int, 0),
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
	}

	statusMutex.Lock()
	uploadStatuses[fileID] = status
	statusMutex.Unlock()

	// 创建临时目录
	chunkDir := filepath.Join(tempDir, fileID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		http.Error(w, "Failed to create chunk directory", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"file_id": fileID,
		"status":  "initialized",
	})
}

func handleChunkUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析URL参数
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	fileID := parts[3]
	chunkNum, err := strconv.Atoi(parts[4])
	if err != nil {
		http.Error(w, "Invalid chunk number", http.StatusBadRequest)
		return
	}

	statusMutex.RLock()
	status, exists := uploadStatuses[fileID]
	statusMutex.RUnlock()

	if !exists {
		http.Error(w, "Upload not initialized", http.StatusNotFound)
		return
	}

	// 检查分片是否已上传
	for _, uploaded := range status.Uploaded {
		if uploaded == chunkNum {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// 保存分片
	chunkPath := filepath.Join(tempDir, fileID, fmt.Sprintf("chunk_%d", chunkNum))
	chunkFile, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, "Failed to create chunk file", http.StatusInternalServerError)
		return
	}
	defer chunkFile.Close()

	// 计算分片的校验和
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(chunkFile, hash), r.Body)
	if err != nil {
		http.Error(w, "Failed to save chunk", http.StatusInternalServerError)
		return
	}

	// 更新状态
	statusMutex.Lock()
	status.Uploaded = append(status.Uploaded, chunkNum)
	status.LastUpdate = time.Now()
	statusMutex.Unlock()

	w.WriteHeader(http.StatusOK)
}

func handleUploadStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID := strings.TrimPrefix(r.URL.Path, "/upload/status/")

	// 检查是否是请求分片状态
	if strings.HasSuffix(r.URL.Path, "/chunks") {
		handleChunkStatus(w, r, fileID)
		return
	}

	statusMutex.RLock()
	status, exists := uploadStatuses[fileID]
	statusMutex.RUnlock()

	if !exists {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// 新增：处理分片状态请求
func handleChunkStatus(w http.ResponseWriter, r *http.Request, fileID string) {
	statusMutex.RLock()
	status, exists := uploadStatuses[fileID]
	statusMutex.RUnlock()

	if !exists {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// 获取所有分片的状态
	chunkDir := filepath.Join(tempDir, fileID)
	chunks := make(map[int]struct {
		Exists bool   `json:"exists"`
		Size   int64  `json:"size"`
		Hash   string `json:"hash,omitempty"`
	})

	// 检查每个分片
	for i := 0; i < status.TotalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%d", i))
		chunkInfo, err := os.Stat(chunkPath)

		chunkStatus := struct {
			Exists bool   `json:"exists"`
			Size   int64  `json:"size"`
			Hash   string `json:"hash,omitempty"`
		}{
			Exists: err == nil,
		}

		if chunkStatus.Exists {
			chunkStatus.Size = chunkInfo.Size()
			// 计算分片的校验和
			if file, err := os.Open(chunkPath); err == nil {
				hash := sha256.New()
				if _, err := io.Copy(hash, file); err == nil {
					chunkStatus.Hash = hex.EncodeToString(hash.Sum(nil))
				}
				file.Close()
			}
		}

		chunks[i] = chunkStatus
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"file_id":      fileID,
		"file_name":    status.FileName,
		"total_size":   status.TotalSize,
		"total_chunks": status.TotalChunks,
		"chunk_size":   status.ChunkSize,
		"chunks":       chunks,
	})
}

func handleUploadComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileID := strings.TrimPrefix(r.URL.Path, "/upload/complete/")

	statusMutex.RLock()
	status, exists := uploadStatuses[fileID]
	statusMutex.RUnlock()

	if !exists {
		http.Error(w, "Upload not found", http.StatusNotFound)
		return
	}

	// 检查是否所有分片都已上传
	if len(status.Uploaded) != status.TotalChunks {
		http.Error(w, "Not all chunks uploaded", http.StatusBadRequest)
		return
	}

	// 合并文件
	finalPath := filepath.Join(uploadDir, status.FileName)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		http.Error(w, "Failed to create final file", http.StatusInternalServerError)
		return
	}
	defer finalFile.Close()

	// 按顺序合并分片
	hash := sha256.New()
	for i := 0; i < status.TotalChunks; i++ {
		chunkPath := filepath.Join(tempDir, fileID, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			http.Error(w, "Failed to read chunk", http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(io.MultiWriter(finalFile, hash), chunkFile)
		chunkFile.Close()
		if err != nil {
			http.Error(w, "Failed to write chunk", http.StatusInternalServerError)
			return
		}
	}

	// 更新状态
	statusMutex.Lock()
	status.Completed = true
	status.Checksum = hex.EncodeToString(hash.Sum(nil))
	statusMutex.Unlock()

	// 清理临时文件
	os.RemoveAll(filepath.Join(tempDir, fileID))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "completed",
		"checksum": status.Checksum,
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取文件路径（可能包含目录）
	filePath := strings.TrimPrefix(r.URL.Path, "/download/")
	if filePath == "" {
		http.Error(w, "File path not provided", http.StatusBadRequest)
		return
	}

	// URL解码
	filePath, err := url.QueryUnescape(filePath)
	if err != nil {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// 构建完整的文件路径
	fullPath := filepath.Join(uploadDir, filePath)

	// 安全检查：确保文件在上传目录内
	if !strings.HasPrefix(fullPath, filepath.Clean(uploadDir)+string(os.PathSeparator)) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// 如果是目录，返回错误
	if fileInfo.IsDir() {
		http.Error(w, "Cannot download directory", http.StatusBadRequest)
		return
	}

	// 设置基本响应头
	filename := filepath.Base(filePath)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Accept-Ranges", "bytes")

	// 如果是HEAD请求，只返回头部
	if r.Method == http.MethodHead {
		return
	}

	// 支持断点续传
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		handleRangeDownload(w, r, fullPath, fileInfo, rangeHeader)
		return
	}

	// 普通下载
	http.ServeFile(w, r, fullPath)
}

func handleRangeDownload(w http.ResponseWriter, r *http.Request, filepath string, fileInfo os.FileInfo, rangeHeader string) {
	// 解析Range头
	var start, end int64
	fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
	if end == 0 {
		end = fileInfo.Size() - 1
	}

	if start >= fileInfo.Size() || end >= fileInfo.Size() || start > end {
		http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// 打开文件
	file, err := os.Open(filepath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 设置响应头
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileInfo.Size()))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusPartialContent)

	// 定位到起始位置
	if _, err := file.Seek(start, 0); err != nil {
		http.Error(w, "Failed to seek file", http.StatusInternalServerError)
		return
	}

	// 发送数据
	_, err = io.CopyN(w, file, end-start+1)
	if err != nil {
		log.Printf("Error sending file: %v", err)
		return
	}
}

func handleListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type FileInfo struct {
		Name     string    `json:"name"`
		Path     string    `json:"path"`
		Size     int64     `json:"size"`
		Modified time.Time `json:"modified"`
		IsDir    bool      `json:"is_dir"`
	}

	var fileInfos []FileInfo

	// 递归遍历上传目录
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过根目录本身
		if path == uploadDir {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(uploadDir, path)
		if err != nil {
			return err
		}

		fileInfos = append(fileInfos, FileInfo{
			Name:     info.Name(),
			Path:     strings.ReplaceAll(relPath, "\\", "/"), // 统一使用斜杠
			Size:     info.Size(),
			Modified: info.ModTime(),
			IsDir:    info.IsDir(),
		})

		return nil
	})

	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileInfos)
}

func generateFileID(filename string, size int64) string {
	hash := sha256.New()
	hash.Write([]byte(filename))
	hash.Write([]byte(fmt.Sprintf("%d", size)))
	hash.Write([]byte(time.Now().String()))
	return hex.EncodeToString(hash.Sum(nil))[:16]
}
