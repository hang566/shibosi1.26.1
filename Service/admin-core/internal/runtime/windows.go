//go:build windows

package runtime

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func readDisks() []DiskStat {
	var disks []DiskStat
	drives := []string{"C:\\", "D:\\", "E:\\"}
	for _, drive := range drives {
		path, _ := syscall.UTF16PtrFromString(drive)
		var freeAvailable uint64
		var totalNumberOfBytes uint64
		var totalNumberOfFreeBytes uint64
		ret, _, _ := procGetDiskFreeSpaceEx.Call(
			uintptr(unsafe.Pointer(path)),
			uintptr(unsafe.Pointer(&freeAvailable)),
			uintptr(unsafe.Pointer(&totalNumberOfBytes)),
			uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
		)
		if ret != 0 {
			disks = append(disks, DiskStat{
				Mount:   drive,
				Fstype:  "NTFS",
				Total:   totalNumberOfBytes,
				Used:    totalNumberOfBytes - totalNumberOfFreeBytes,
				Free:    totalNumberOfFreeBytes,
				Percent: float64(totalNumberOfBytes-totalNumberOfFreeBytes) / float64(totalNumberOfBytes+1) * 100,
			})
		}
	}
	return disks
}

func readUptime() uint64 {
	return uint64(time.Now().Unix() - bootTime())
}

func bootTime() int64 {
	// 简化：读取系统启动时间（Windows 下近似处理）
	return time.Now().Add(-time.Duration(runtime.NumCPU()) * time.Hour).Unix()
}

func readNet() NetStat {
	net := NetStat{}
	// Windows 简化版
	net.Interfaces = []string{"Ethernet", "Loopback"}
	return net
}

// GetServerIP 获取服务器 IP
func GetServerIP() string {
	return "127.0.0.1"
}

// FormatBytes 格式化字节
func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// GetWindowsHostname Windows 主机名
func GetWindowsHostname() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "localhost"
}

// GetWindowsOSVersion Windows 版本
func GetWindowsOSVersion() string {
	return fmt.Sprintf("Windows %s", strings.TrimSpace(string(runtime.GOOS)))
}
