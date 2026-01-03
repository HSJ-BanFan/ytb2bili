//go:build windows

package utils

import (
	"fmt"
	"syscall"
	"unsafe"
)

// getDiskFreeBytesWindows 使用 Windows API 获取磁盘空间
func (sm *SystemMonitor) getDiskFreeBytesWindows(dir string) (uint64, error) {
	// 加载 kernel32.dll
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW := kernel32.NewProc("GetDiskFreeSpaceExW")

	// 转换路径为 UTF-16
	dirPtr, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, fmt.Errorf("路径转换失败: %w", err)
	}

	var freeBytesAvailable uint64
	var totalBytes uint64
	var totalFreeBytes uint64

	// 调用 Windows API
	ret, _, callErr := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("GetDiskFreeSpaceExW 调用失败: %v", callErr)
	}

	return freeBytesAvailable, nil
}

// getDiskFreeBytesUnix 在 Windows 平台返回占位值
// 此函数不会被调用，因为 getDiskFreeBytes 会调用 getDiskFreeBytesWindows
func (sm *SystemMonitor) getDiskFreeBytesUnix(dir string) (uint64, error) {
	return 0, nil
}
