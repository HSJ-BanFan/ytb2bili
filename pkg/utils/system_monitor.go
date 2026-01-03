package utils

import (
	"fmt"
	"os"
	"runtime"
)

// SystemMonitor 系统监控工具
type SystemMonitor struct {
	minDiskFreeGB float64 // 最小剩余磁盘空间（GB）
}

// NewSystemMonitor 创建系统监控器
func NewSystemMonitor(minDiskFreeGB float64) *SystemMonitor {
	if minDiskFreeGB <= 0 {
		minDiskFreeGB = 1.0 // 默认至少保留1GB
	}
	return &SystemMonitor{
		minDiskFreeGB: minDiskFreeGB,
	}
}

// CheckDiskSpace 检查磁盘剩余空间
// 返回 (剩余空间GB, 是否有足够空间, 错误)
func (sm *SystemMonitor) CheckDiskSpace(dir string) (freeGB float64, ok bool, err error) {
	// 获取目录信息
	statInfo, err := os.Stat(dir)
	if err != nil {
		return 0, false, fmt.Errorf("无法访问目录 %s: %w", dir, err)
	}

	// 如果是文件，获取其所在目录
	if !statInfo.IsDir() {
		dir = getDir(dir)
	}

	// 获取磁盘使用情况
	freeBytes, err := sm.getDiskFreeBytes(dir)
	if err != nil {
		return 0, false, fmt.Errorf("获取磁盘空间失败: %w", err)
	}

	freeGB = float64(freeBytes) / (1024 * 1024 * 1024)
	ok = freeGB >= sm.minDiskFreeGB

	return freeGB, ok, nil
}

// GetMemoryUsage 获取内存使用情况
// 返回 (已分配MB, 已分配GB, 系统总内存GB)
func (sm *SystemMonitor) GetMemoryUsage() (allocMB, allocGB, sysGB uint64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB = m.Alloc / 1024 / 1024
	allocGB = allocMB / 1024
	sysGB = m.Sys / 1024 / 1024 / 1024

	return allocMB, allocGB, sysGB
}

// IsDiskSpaceEnough 检查磁盘空间是否足够
func (sm *SystemMonitor) IsDiskSpaceEnough(dir string) bool {
	_, ok, err := sm.CheckDiskSpace(dir)
	if err != nil {
		// 检查失败时，为了安全起见返回true（不阻塞任务）
		// 但记录错误日志
		fmt.Printf("⚠️ 磁盘空间检查失败: %v\n", err)
		return true
	}
	return ok
}

// getDiskFreeBytes 获取磁盘剩余字节数（跨平台）
func (sm *SystemMonitor) getDiskFreeBytes(dir string) (uint64, error) {
	if runtime.GOOS == "windows" {
		return sm.getDiskFreeBytesWindows(dir)
	}
	return sm.getDiskFreeBytesUnix(dir)
}

// getDiskFreeBytesWindows 在 system_monitor_windows.go 中实现

// getDiskFreeBytesUnix 在 system_monitor_unix.go 中实现

// GetSystemInfo 获取系统信息
func (sm *SystemMonitor) GetSystemInfo() map[string]interface{} {
	allocMB, allocGB, sysGB := sm.GetMemoryUsage()

	return map[string]interface{}{
		"memory_alloc_mb":  allocMB,
		"memory_alloc_gb":  allocGB,
		"memory_sys_gb":    sysGB,
		"goroutines":       runtime.NumGoroutine(),
		"cpu_num":          runtime.NumCPU(),
		"min_disk_free_gb": sm.minDiskFreeGB,
		"go_version":       runtime.Version(),
		"operating_system": runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// PrintSystemInfo 打印系统信息（用于调试）
func (sm *SystemMonitor) PrintSystemInfo() {
	info := sm.GetSystemInfo()
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("📊 系统信息")
	fmt.Println("═══════════════════════════════════════")
	fmt.Printf("Go 版本: %s\n", info["go_version"])
	fmt.Printf("操作系统: %s\n", info["operating_system"])
	fmt.Printf("CPU 核心数: %v\n", info["cpu_num"])
	fmt.Printf("Goroutines: %v\n", info["goroutines"])
	fmt.Printf("内存使用: %v MB (%v GB)\n", info["memory_alloc_mb"], info["memory_alloc_gb"])
	fmt.Printf("系统内存: %v GB\n", info["memory_sys_gb"])
	fmt.Printf("最小磁盘保留: %v GB\n", info["min_disk_free_gb"])
	fmt.Println("═══════════════════════════════════════")
}

// getDir 获取文件路径的目录部分
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			return path[:i]
		}
	}
	return "."
}
