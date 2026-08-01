//go:build linux || darwin

package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func readDisks() []DiskStat {
	var disks []DiskStat
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return disks
	}
	defer f.Close()

	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mount := fields[1]
		fs := fields[2]
		if fs != "ext4" && fs != "ext3" && fs != "xfs" && fs != "btrfs" && fs != "zfs" && fs != "overlay" {
			continue
		}
		if seen[mount] {
			continue
		}
		seen[mount] = true

		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		pct := float64(used) / float64(total+1) * 100
		disks = append(disks, DiskStat{
			Mount: mount, Fstype: fs, Total: total, Used: used, Free: free, Percent: pct,
		})
	}
	return disks
}

func readUptime() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
			return uint64(v)
		}
	}
	return 0
}

func readNet() NetStat {
	net := NetStat{}
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return net
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		if line <= 2 {
			continue
		}
		text := scanner.Text()
		colon := strings.Index(text, ":")
		if colon < 0 {
			continue
		}
		iface := strings.TrimSpace(text[:colon])
		fields := strings.Fields(text[colon+1:])
		if len(fields) < 16 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		net.BytesRecv += rx
		net.BytesSent += tx
		if iface != "lo" {
			net.Interfaces = append(net.Interfaces, iface)
		}
	}
	return net
}

// GetServerIP 获取服务器网卡 IP（返回第一个非 loopback IPv4）
func GetServerIP() string {
	data, err := os.ReadFile("/proc/net/fib_trie")
	if err != nil {
		return "127.0.0.1"
	}
	lines := strings.Split(string(data), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "|-- ") && strings.Contains(l, "LOCAL") {
			ip := strings.TrimPrefix(l, "|-- ")
			ip = strings.TrimSpace(strings.TrimSuffix(ip, " LOCAL"))
			if ip != "127.0.0.1" && !strings.HasPrefix(ip, "169.254.") {
				return ip
			}
		}
	}
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
