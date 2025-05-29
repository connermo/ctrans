//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.MustLoadDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.MustFindProc("GetDiskFreeSpaceExW")
)

// checkDiskSpace 检查磁盘空间（Windows系统版本）
func checkDiskSpace(requiredSize int64) error {
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	// 将路径转换为UTF-16
	pathPtr, err := syscall.UTF16PtrFromString(uploadDir)
	if err != nil {
		return fmt.Errorf("failed to convert path: %v", err)
	}

	// 调用 GetDiskFreeSpaceEx
	ret, _, err := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		return fmt.Errorf("failed to get disk space info: %v", err)
	}

	// 检查是否有足够的空间（文件大小 + 最小剩余空间）
	if uint64(requiredSize)+minDiskSpace > freeBytesAvailable {
		return fmt.Errorf("not enough disk space. Required: %d bytes, Available: %d bytes",
			requiredSize+minDiskSpace, freeBytesAvailable)
	}

	return nil
}
