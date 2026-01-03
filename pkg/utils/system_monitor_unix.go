//go:build !windows

package utils

import (
	"fmt"
	"syscall"
)

// getDiskFreeBytesWindows 在非 Windows 平台返回占位值
// 此函数不会被调用，因为 getDiskFreeBytes 会调用 getDiskFreeBytesUnix
func (sm *SystemMonitor) getDiskFreeBytesWindows(dir string) (uint64, error) {
	return 0, nil
}

// getDiskFreeBytesUnix 使用 syscall.Statfs 获取磁盘空间 (Linux/macOS)
func (sm *SystemMonitor) getDiskFreeBytesUnix(dir string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, fmt.Errorf("statfs 调用失败: %w", err)
	}
	// Bavail = 非特权用户可用的块数, Bsize = 块大小
	return stat.Bavail * uint64(stat.Bsize), nil
}
