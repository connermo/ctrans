//go:build !windows

package main

import (
	"fmt"
	"syscall"
)

// checkDiskSpace 检查磁盘空间（Unix系统版本）
func checkDiskSpace(requiredSize int64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(uploadDir, &stat); err != nil {
		return fmt.Errorf("failed to get disk space info: %v", err)
	}

	// 计算可用空间（以字节为单位）
	availableSpace := stat.Bavail * uint64(stat.Bsize)

	// 检查是否有足够的空间（文件大小 + 最小剩余空间）
	if uint64(requiredSize)+minDiskSpace > availableSpace {
		return fmt.Errorf("not enough disk space. Required: %d bytes, Available: %d bytes",
			requiredSize+minDiskSpace, availableSpace)
	}

	return nil
}
