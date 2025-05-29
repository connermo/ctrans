package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

const (
	chunkSize  = 10 * 1024 * 1024 // 10MB per chunk
	maxRetries = 3
	stateDir   = ".upload_state" // 状态文件目录
	authHeader = "X-Service-Key" // 认证头
)

var (
	stateMutex sync.Mutex // 用于保护状态文件的并发访问
)

type UploadState struct {
	FileID      string    `json:"file_id"`
	FileName    string    `json:"file_name"`
	FilePath    string    `json:"file_path"`   // 本地文件路径
	ServerAddr  string    `json:"server_addr"` // 服务器地址
	TotalSize   int64     `json:"total_size"`
	TotalChunks int       `json:"total_chunks"`
	Uploaded    []int     `json:"uploaded_chunks"`
	StartTime   time.Time `json:"start_time"`
	LastUpdate  time.Time `json:"last_update"`
	Completed   bool      `json:"completed"`
	Checksum    string    `json:"checksum,omitempty"`
}

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

type FileInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type ChunkStatus struct {
	Exists bool   `json:"exists"`
	Size   int64  `json:"size"`
	Hash   string `json:"hash,omitempty"`
}

type ServerChunkStatus struct {
	FileID      string              `json:"file_id"`
	FileName    string              `json:"file_name"`
	TotalSize   int64               `json:"total_size"`
	TotalChunks int                 `json:"total_chunks"`
	ChunkSize   int64               `json:"chunk_size"`
	Chunks      map[int]ChunkStatus `json:"chunks"`
}

// 添加新的类型用于跟踪上传速度
type speedTracker struct {
	startTime time.Time
	bytes     int64
	lastBytes int64
	lastTime  time.Time
}

func (s *speedTracker) update(n int64) {
	s.bytes += n
	now := time.Now()
	if now.Sub(s.lastTime) >= time.Second {
		s.lastBytes = s.bytes
		s.lastTime = now
	}
}

func (s *speedTracker) speed() string {
	elapsed := time.Since(s.lastTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	speed := float64(s.bytes-s.lastBytes) / elapsed
	return formatSpeed(speed)
}

func (s *speedTracker) averageSpeed() string {
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}
	speed := float64(s.bytes) / elapsed
	return formatSpeed(speed)
}

func formatSpeed(bytesPerSecond float64) string {
	const unit = 1024
	if bytesPerSecond < unit {
		return fmt.Sprintf("%.1f B/s", bytesPerSecond)
	}
	div, exp := float64(unit), 0
	for n := bytesPerSecond / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSecond/div, "KMGTPE"[exp])
}

// 自定义进度条写入器 - 用于下载时同时更新进度和写入文件
type downloadProgressWriter struct {
	fileWriter io.Writer
	tracker    *speedTracker
	bar        *progressbar.ProgressBar
	fileName   string
}

func (dpw *downloadProgressWriter) Write(p []byte) (int, error) {
	// 先写入文件
	n, err := dpw.fileWriter.Write(p)
	if err != nil {
		return n, err
	}

	// 然后更新进度
	if n > 0 {
		dpw.tracker.update(int64(n))
		dpw.bar.Add(n)
		dpw.bar.Describe(fmt.Sprintf("Downloading %s (%s, avg: %s)",
			dpw.fileName,
			dpw.tracker.speed(),
			dpw.tracker.averageSpeed()))
	}
	return n, err
}

// 自定义进度条写入器 - 用于上传时更新进度
type progressWriter struct {
	writer    io.Writer
	tracker   *speedTracker
	bar       *progressbar.ProgressBar
	fileName  string
	startTime time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.tracker.update(int64(n))
		pw.bar.Describe(fmt.Sprintf("Uploading %s (%s, avg: %s)",
			pw.fileName,
			pw.tracker.speed(),
			pw.tracker.averageSpeed()))
	}
	return n, err
}

// 创建带认证的HTTP客户端
func createClient(serverKey string) *http.Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 如果设置了服务密钥，添加默认传输器
	if serverKey != "" {
		client.Transport = &authTransport{
			key:  serverKey,
			base: http.DefaultTransport,
		}
	}

	return client
}

// 认证传输器
type authTransport struct {
	key  string
	base http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(authHeader, t.key)
	return t.base.RoundTrip(req)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  Upload:   %s <local-file> <server:port>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Download: %s <server:port>/<filename> [local-path]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  List:     %s <server:port>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}

	// 定义可选参数
	serverKey := flag.String("key", "", "Service key for authentication (optional)")
	resumeUpload := flag.String("resume", "", "Resume upload with file ID (optional)")
	help := flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	// 获取非标志参数
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// 创建HTTP客户端
	client := createClient(*serverKey)

	// 创建状态目录
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		log.Fatal("Failed to create state directory:", err)
	}

	// 解析命令类型
	if *resumeUpload != "" {
		// Resume模式
		if len(args) != 1 {
			fmt.Fprintf(os.Stderr, "Error: Resume mode requires server:port\n")
			flag.Usage()
			os.Exit(1)
		}
		serverAddr := parseServerAddr(args[0])
		resumeUploadFile(serverAddr, *resumeUpload, client)
		return
	}

	if len(args) == 1 {
		// 列表模式: ctrans server:port
		arg := args[0]
		if !strings.Contains(arg, ":") {
			fmt.Fprintf(os.Stderr, "Error: Invalid server format. Use server:port\n")
			flag.Usage()
			os.Exit(1)
		}
		serverAddr := parseServerAddr(arg)
		list(serverAddr, client)
	} else if len(args) == 2 {
		// 两个参数：可能是上传或下载
		first := args[0]
		second := args[1]

		if strings.Contains(first, ":") && strings.Contains(first, "/") {
			// 下载模式: ctrans server:port/filename localpath
			downloadFromRemote(first, second, client)
		} else if strings.Contains(second, ":") && !strings.Contains(second, "/") {
			// 上传模式: ctrans localfile server:port
			uploadFile := first
			serverAddr := parseServerAddr(second)

			// 检查是否有未完成的上传任务
			if state := findIncompleteUpload(uploadFile, serverAddr, client); state != nil {
				color.Yellow("Found incomplete upload for %s", uploadFile)
				color.Yellow("Resuming upload with file ID: %s", state.FileID)
				uploadChunks(serverAddr, state.FileID, state.FilePath, state.TotalSize, client)
			} else {
				upload(serverAddr, uploadFile, client)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: Invalid arguments. Check usage.\n")
			flag.Usage()
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: Too many arguments\n")
		flag.Usage()
		os.Exit(1)
	}
}

// 解析服务器地址，确保格式正确
func parseServerAddr(addr string) string {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return addr
}

// 处理下载命令：ctrans server:port/filename localpath
func downloadFromRemote(remote string, localPath string, client *http.Client) {
	// 解析远程路径: server:port/filename
	parts := strings.SplitN(remote, "/", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Error: Invalid remote path format. Use server:port/filename\n")
		os.Exit(1)
	}

	serverAddr := parseServerAddr(parts[0])
	filename := parts[1]

	// 如果没有指定本地路径，使用文件名
	if localPath == "" {
		localPath = filename
	}

	downloadFile(serverAddr, filename, localPath, client)
}

// 修改下载函数以支持自定义本地路径
func downloadFile(serverAddr, filename, localPath string, client *http.Client) {
	// 获取文件信息
	resp, err := client.Head(fmt.Sprintf("%s/download/%s", serverAddr, filename))
	if err != nil {
		log.Fatal("Error getting file info:", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("File not found: %s", resp.Status)
	}

	fileSize := resp.ContentLength
	if fileSize == -1 {
		log.Fatal("Server did not provide file size")
	}

	// 创建目标文件
	out, err := os.Create(localPath)
	if err != nil {
		log.Fatal("Error creating file:", err)
	}
	defer out.Close()

	// 创建速度跟踪器
	tracker := &speedTracker{
		startTime: time.Now(),
		lastTime:  time.Now(),
	}

	// 创建进度条
	bar := progressbar.NewOptions64(
		fileSize,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %s", filename)),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
	)

	// 创建进度写入器
	progressWriter := &downloadProgressWriter{
		fileWriter: out,
		tracker:    tracker,
		bar:        bar,
		fileName:   filename,
	}

	// 检查是否存在部分下载的文件
	fileInfo, err := out.Stat()
	if err == nil && fileInfo.Size() > 0 {
		// 获取已下载的大小
		downloaded := fileInfo.Size()
		if downloaded >= fileSize {
			color.Green("File already downloaded!")
			return
		}

		// 设置Range头
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/download/%s", serverAddr, filename), nil)
		if err != nil {
			log.Fatal("Error creating request:", err)
		}
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloaded))

		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Error resuming download:", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusPartialContent {
			log.Fatal("Server does not support resume")
		}

		bar.Add64(downloaded)
		_, err = io.Copy(progressWriter, resp.Body)
	} else {
		// 从头开始下载
		resp, err := client.Get(fmt.Sprintf("%s/download/%s", serverAddr, filename))
		if err != nil {
			log.Fatal("Error downloading file:", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Fatalf("Download failed: %s - %s", resp.Status, string(body))
		}

		_, err = io.Copy(progressWriter, resp.Body)
	}

	color.Green("Download completed successfully!")
}

func findIncompleteUpload(filePath, serverAddr string, client *http.Client) *UploadState {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}

	// 读取状态文件
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			statePath := filepath.Join(stateDir, entry.Name())
			data, err := os.ReadFile(statePath)
			if err != nil {
				continue
			}

			var state UploadState
			if err := json.Unmarshal(data, &state); err != nil {
				// 删除无效的状态文件
				os.Remove(statePath)
				continue
			}

			// 忽略fileID为空的状态文件
			if state.FileID == "" {
				log.Printf("Warning: Found state file with empty file ID, removing: %s", statePath)
				os.Remove(statePath)
				continue
			}

			if state.FilePath == absPath && state.ServerAddr == serverAddr && !state.Completed {
				// 获取服务器上的分片状态
				serverStatus, err := getServerChunkStatus(serverAddr, state.FileID, client)
				if err != nil {
					log.Printf("Warning: Failed to get server chunk status: %v", err)
					// 如果服务器上没有这个上传会话，删除本地状态文件
					log.Printf("Removing invalid state file: %s", statePath)
					os.Remove(statePath)
					continue
				}

				// 检查服务器状态是否有效
				if serverStatus != nil && serverStatus.FileID == state.FileID {
					return &state
				}
			}
		}
	}
	return nil
}

func saveUploadState(state *UploadState) error {
	// 检查fileID是否为空
	if state.FileID == "" {
		return fmt.Errorf("cannot save state with empty file ID")
	}

	// 生成状态文件名
	stateFile := filepath.Join(stateDir, state.FileID+".json")

	// 更新最后修改时间
	state.LastUpdate = time.Now()

	// 保存状态
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0644)
}

func deleteUploadState(fileID string) error {
	stateFile := filepath.Join(stateDir, fileID+".json")
	return os.Remove(stateFile)
}

func upload(serverAddr, filePath string, client *http.Client) {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatal("Error getting file info:", err)
	}

	// 获取文件的绝对路径
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Fatal("Error getting absolute path:", err)
	}

	// 初始化上传
	initResp, err := initUpload(serverAddr, filePath, fileInfo.Size(), client)
	if err != nil {
		log.Fatal("Failed to initialize upload:", err)
	}

	// 验证返回的fileID不为空
	if initResp.FileID == "" {
		log.Fatal("Server returned empty file ID")
	}

	// 获取文件名
	_, fileName := filepath.Split(absPath)

	// 创建上传状态
	state := &UploadState{
		FileID:      initResp.FileID,
		FileName:    fileName,
		FilePath:    absPath,
		ServerAddr:  serverAddr,
		TotalSize:   fileInfo.Size(),
		TotalChunks: int((fileInfo.Size() + chunkSize - 1) / chunkSize),
		Uploaded:    make([]int, 0),
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
	}

	// 只有在fileID不为空时才保存状态
	if state.FileID != "" {
		if err := saveUploadState(state); err != nil {
			log.Printf("Warning: Failed to save upload state: %v", err)
		}
	}

	// 开始上传分片
	uploadChunks(serverAddr, state.FileID, filePath, fileInfo.Size(), client)
}

func initUpload(serverAddr, filePath string, fileSize int64, client *http.Client) (struct {
	FileID string `json:"file_id"`
}, error) {
	// 获取文件名
	_, fileName := filepath.Split(filePath)

	reqBody := struct {
		FileName  string `json:"file_name"`
		TotalSize int64  `json:"total_size"`
	}{
		FileName:  fileName,
		TotalSize: fileSize,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return struct {
			FileID string `json:"file_id"`
		}{}, err
	}

	resp, err := client.Post(serverAddr+"/upload/init", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return struct {
			FileID string `json:"file_id"`
		}{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return struct {
			FileID string `json:"file_id"`
		}{}, fmt.Errorf("init failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return struct {
			FileID string `json:"file_id"`
		}{}, err
	}

	return result, nil
}

func uploadChunks(serverAddr, fileID, filePath string, fileSize int64, client *http.Client) {
	// 获取上传状态
	status, err := getUploadStatus(serverAddr, fileID, client)
	if err != nil {
		log.Fatal("Failed to get upload status:", err)
	}

	// 获取服务器上的分片状态
	serverStatus, err := getServerChunkStatus(serverAddr, fileID, client)
	if err != nil {
		log.Printf("Warning: Failed to get server chunk status: %v", err)
	} else {
		// 比较本地文件和服务器状态
		neededChunks := compareChunks(filePath, serverStatus)
		if len(neededChunks) > 0 {
			color.Yellow("Found %d chunks that need to be uploaded", len(neededChunks))
			status.Uploaded = neededChunks
		}
	}

	// 加载本地状态
	stateFile := filepath.Join(stateDir, fileID+".json")
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		log.Printf("Warning: Failed to read state file: %v", err)
	} else {
		var localState UploadState
		if err := json.Unmarshal(stateData, &localState); err == nil {
			// 使用本地状态更新服务器状态
			status.Uploaded = localState.Uploaded
		}
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer file.Close()

	// 创建速度跟踪器
	tracker := &speedTracker{
		startTime: time.Now(),
		lastTime:  time.Now(),
	}

	// 获取文件名用于显示
	_, fileName := filepath.Split(filePath)

	// 创建进度条
	bar := progressbar.NewOptions64(
		fileSize,
		progressbar.OptionSetDescription(fmt.Sprintf("Uploading %s", fileName)),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
	)

	// 创建进度写入器
	progressWriter := &progressWriter{
		writer:    file,
		tracker:   tracker,
		bar:       bar,
		fileName:  fileName,
		startTime: time.Now(),
	}

	// 创建等待组和错误通道
	var wg sync.WaitGroup
	errChan := make(chan error, status.TotalChunks)
	semaphore := make(chan struct{}, 5) // 限制并发数

	// 更新本地状态
	state := &UploadState{
		FileID:      fileID,
		FileName:    status.FileName,
		FilePath:    filePath,
		ServerAddr:  serverAddr,
		TotalSize:   status.TotalSize,
		TotalChunks: status.TotalChunks,
		Uploaded:    status.Uploaded,
		StartTime:   status.StartTime,
		LastUpdate:  time.Now(),
	}

	// 上传分片
	for i := 0; i < status.TotalChunks; i++ {
		// 检查分片是否已上传
		chunkUploaded := false
		for _, uploaded := range status.Uploaded {
			if uploaded == i {
				chunkUploaded = true
				bar.Add64(chunkSize)
				break
			}
		}
		if chunkUploaded {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量

		go func(chunkNum int) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量

			// 计算分片大小
			start := int64(chunkNum) * chunkSize
			end := start + chunkSize
			if end > fileSize {
				end = fileSize
			}
			chunkSize := end - start

			// 读取分片数据
			chunk := make([]byte, chunkSize)
			if _, err := file.ReadAt(chunk, start); err != nil {
				errChan <- fmt.Errorf("error reading chunk %d: %v", chunkNum, err)
				return
			}

			// 上传分片
			for retry := 0; retry < maxRetries; retry++ {
				req, err := http.NewRequest("POST",
					fmt.Sprintf("%s/upload/chunk/%s/%d", serverAddr, fileID, chunkNum),
					bytes.NewReader(chunk))
				if err != nil {
					errChan <- fmt.Errorf("error creating request for chunk %d: %v", chunkNum, err)
					return
				}

				resp, err := client.Do(req)
				if err != nil {
					if retry == maxRetries-1 {
						errChan <- fmt.Errorf("error uploading chunk %d: %v", chunkNum, err)
						return
					}
					time.Sleep(time.Second * time.Duration(retry+1))
					continue
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					if retry == maxRetries-1 {
						errChan <- fmt.Errorf("error uploading chunk %d: %s", chunkNum, resp.Status)
						return
					}
					time.Sleep(time.Second * time.Duration(retry+1))
					continue
				}

				// 更新状态
				stateMutex.Lock()
				state.Uploaded = append(state.Uploaded, chunkNum)
				state.LastUpdate = time.Now()
				if err := saveUploadState(state); err != nil {
					log.Printf("Warning: Failed to save upload state: %v", err)
				}
				stateMutex.Unlock()

				// 使用进度写入器更新进度
				progressWriter.Write(chunk)
				break
			}
		}(i)
	}

	// 等待所有上传完成
	wg.Wait()
	close(errChan)

	// 检查错误
	for err := range errChan {
		log.Printf("Upload error: %v", err)
		log.Fatal("Upload failed, you can resume later by running the same upload command")
	}

	// 完成上传
	completeUpload(serverAddr, fileID, client)

	// 删除状态文件
	if err := deleteUploadState(fileID); err != nil {
		log.Printf("Warning: Failed to delete state file: %v", err)
	}
}

func getUploadStatus(serverAddr, fileID string, client *http.Client) (*UploadStatus, error) {
	resp, err := client.Get(fmt.Sprintf("%s/upload/status/%s", serverAddr, fileID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get status: %s - %s", resp.Status, string(body))
	}

	var status UploadStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func completeUpload(serverAddr, fileID string, client *http.Client) {
	resp, err := client.Post(fmt.Sprintf("%s/upload/complete/%s", serverAddr, fileID), "", nil)
	if err != nil {
		log.Fatal("Failed to complete upload:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Failed to complete upload: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Status   string `json:"status"`
		Checksum string `json:"checksum"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatal("Failed to decode response:", err)
	}

	color.Green("Upload completed successfully!")
	color.Cyan("File checksum: %s", result.Checksum)
}

func resumeUploadFile(serverAddr, fileID string, client *http.Client) {
	// 获取上传状态
	status, err := getUploadStatus(serverAddr, fileID, client)
	if err != nil {
		log.Fatal("Failed to get upload status:", err)
	}

	if status.Completed {
		color.Green("Upload already completed!")
		return
	}

	// 继续上传
	uploadChunks(serverAddr, fileID, status.FileName, status.TotalSize, client)
}

func list(serverAddr string, client *http.Client) {
	resp, err := client.Get(serverAddr + "/files")
	if err != nil {
		log.Fatal("Error listing files:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("List failed: %s - %s", resp.Status, string(body))
	}

	var files []FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		log.Fatal("Error parsing response:", err)
	}

	if len(files) == 0 {
		fmt.Println("No files available")
		return
	}

	fmt.Println("\nAvailable files:")
	fmt.Println("----------------")
	for _, file := range files {
		fmt.Printf("Name: %s\n", color.CyanString(file.Name))
		fmt.Printf("Size: %s\n", formatSize(file.Size))
		fmt.Printf("Modified: %s\n", file.Modified.Format(time.RFC3339))
		fmt.Println("----------------")
	}
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func getServerChunkStatus(serverAddr, fileID string, client *http.Client) (*ServerChunkStatus, error) {
	resp, err := client.Get(fmt.Sprintf("%s/upload/status/%s/chunks", serverAddr, fileID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get chunk status: %s - %s", resp.Status, string(body))
	}

	var status ServerChunkStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

func compareChunks(filePath string, serverStatus *ServerChunkStatus) []int {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file for comparison: %v", err)
		return nil
	}
	defer file.Close()

	var neededChunks []int
	hash := sha256.New()
	buffer := make([]byte, chunkSize)

	for i := 0; i < serverStatus.TotalChunks; i++ {
		// 计算当前分片的起始位置
		start := int64(i) * serverStatus.ChunkSize
		end := start + serverStatus.ChunkSize
		if end > serverStatus.TotalSize {
			end = serverStatus.TotalSize
		}
		chunkSize := end - start

		// 读取分片数据
		if _, err := file.Seek(start, 0); err != nil {
			log.Printf("Error seeking to chunk %d: %v", i, err)
			continue
		}

		n, err := io.ReadFull(file, buffer[:chunkSize])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Printf("Error reading chunk %d: %v", i, err)
			continue
		}

		// 计算分片校验和
		hash.Reset()
		hash.Write(buffer[:n])
		chunkHash := hex.EncodeToString(hash.Sum(nil))

		// 检查服务器上的分片状态
		serverChunk, exists := serverStatus.Chunks[i]
		if !exists || !serverChunk.Exists || serverChunk.Hash != chunkHash {
			neededChunks = append(neededChunks, i)
		}
	}

	return neededChunks
}
