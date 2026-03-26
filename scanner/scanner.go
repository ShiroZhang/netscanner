package scanner

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ScanResult 表示单个IP的扫描结果
type ScanResult struct {
	IP        string `json:"ip"`
	Status    string `json:"status"`    // "online", "offline"
	Method    string `json:"method"`    // "arp", "icmp"
	MAC       string `json:"mac"`       // MAC地址（如果检测到）
	Latency   int    `json:"latency"`   // 响应时间（毫秒）
	Timestamp string `json:"timestamp"` // 扫描时间
}

// ScanOptions 扫描配置选项
type ScanOptions struct {
	StartIP    string
	EndIP      string
	Timeout    time.Duration
	Concurrent int
	UseARP     bool // 是否使用ARP扫描
	UseICMP    bool // 是否使用ICMP Ping
}

// Scan 扫描指定范围内的IP
func Scan(opts ScanOptions) ([]ScanResult, error) {
	startIP := net.ParseIP(opts.StartIP)
	endIP := net.ParseIP(opts.EndIP)

	if startIP == nil || endIP == nil {
		return nil, fmt.Errorf("无效的IP地址")
	}

	// 生成IP列表
	ips, err := generateIPList(startIP, endIP)
	if err != nil {
		return nil, err
	}

	// 执行并发扫描
	results := make([]ScanResult, len(ips))
	var wg sync.WaitGroup

	// 使用带缓冲的channel控制并发数
	sem := make(chan struct{}, opts.Concurrent)

	for i, ip := range ips {
		wg.Add(1)
		go func(idx int, ipStr string) {
			defer wg.Done()
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			results[idx] = checkIPOnline(ipStr, opts)
		}(i, ip)
	}

	wg.Wait()
	return results, nil
}

// checkIPOnline 检查IP是否在线（改进的混合检测方案）
func checkIPOnline(ip string, opts ScanOptions) ScanResult {
	startTime := time.Now()
	result := ScanResult{
		IP:        ip,
		Status:    "offline",
		Method:    "none",
		MAC:       "",
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 策略1：先用ICMP Ping（会触发ARP请求，同时检测在线状态）
	if opts.UseICMP {
		latency, online := pingIP(ip, opts.Timeout)
		if online {
			result.Status = "online"
			result.Method = "icmp"
			result.Latency = latency

			// 尝试获取MAC地址
			if mac := getMACAddress(ip); mac != "" {
				result.MAC = mac
				result.Method = "arp" // 如果能获取MAC，优先标记为ARP
			}

			return result
		}
	}

	// 策略2：主动ARP检测
	// 如果ICMP没有开启，需要先发一个ping来触发ARP缓存填充
	if opts.UseARP {
		if !opts.UseICMP {
			// 先发一个快速ping来触发ARP缓存（不关心结果）
			triggerARPCache(ip)
			time.Sleep(100 * time.Millisecond) // 短暂等待ARP表更新
		}
		if mac := getMACAddress(ip); mac != "" {
			result.Status = "online"
			result.Method = "arp"
			result.MAC = mac
			result.Latency = int(time.Since(startTime).Milliseconds())
			return result
		}
	}

	// 如果都失败，标记为离线
	result.Latency = int(opts.Timeout.Milliseconds())
	return result
}

// triggerARPCache 发送一个快速ping以触发ARP缓存填充
func triggerARPCache(ip string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("ping", "-n", "1", "-w", "200", ip)
	default:
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	// 不关心结果，只是触发ARP
	_ = cmd.Run()
}

// pingIP 使用系统ping命令检测IP
func pingIP(ip string, timeout time.Duration) (int, bool) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		timeoutMs := int(timeout.Milliseconds())
		// Windows ping: -n 1 (发送1个包), -w (超时毫秒)
		cmd = exec.Command("ping", "-n", "1", "-w", fmt.Sprintf("%d", timeoutMs), ip)
	case "linux", "darwin":
		timeoutSec := timeout.Seconds()
		// Linux/Mac ping: -c 1 (发送1个包), -W (超时秒)
		cmd = exec.Command("ping", "-c", "1", "-W", fmt.Sprintf("%.1f", timeoutSec), ip)
	default:
		return 0, false
	}

	// 执行命令
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// 始终检查输出内容来判断是否在线
	// 不能仅依赖 err == nil，因为 Windows ping 即使超时也可能返回 exit code 0
	online := isPingSuccessful(outputStr)

	if !online {
		// 如果输出中没有成功标志，则认为离线
		_ = err // err 可能非 nil（ping 失败时返回非零退出码）
		return 0, false
	}

	// 解析输出获取延迟时间
	latency := parsePingLatency(outputStr)
	return latency, true
}

// isPingSuccessful 检查ping输出是否表示成功
func isPingSuccessful(output string) bool {
	outputLower := strings.ToLower(output)

	// 先检查失败关键字（优先排除）
	failureKeywords := []string{
		"请求超时",           // Windows 中文
		"无法访问目标主机",       // Windows 中文
		"一般故障",           // Windows 中文
		"request timed out", // Windows 英文
		"destination host unreachable", // Windows/Linux 英文
		"general failure",              // Windows 英文
		"100% packet loss",             // Linux 英文
		"100% 丢失",                     // Windows 中文
	}
	for _, keyword := range failureKeywords {
		if strings.Contains(outputLower, strings.ToLower(keyword)) {
			return false
		}
	}

	// 检查成功关键字
	successKeywords := []string{
		"来自",          // Windows 中文: "来自 x.x.x.x 的回复"
		"reply from",  // Windows 英文
		"bytes from",  // Linux: "64 bytes from x.x.x.x"
		"1 received",  // Linux: "1 packets transmitted, 1 received"
		"1 packets received", // macOS 变体
		"ttl=",        // 通用：TTL 字段出现意味着收到了回复
	}
	for _, keyword := range successKeywords {
		if strings.Contains(outputLower, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// parsePingLatency 从ping输出中解析延迟时间
func parsePingLatency(output string) int {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// === Windows 中文格式 ===
		// 正常: "来自 192.168.1.1 的回复: 字节=32 时间=14ms TTL=64"
		// 快速: "来自 192.168.1.1 的回复: 字节=32 时间<1ms TTL=64"

		// 检查 "时间=" 格式
		if idx := strings.Index(line, "时间="); idx >= 0 {
			timePart := line[idx+len("时间="):] // 正确跳过 UTF-8 字节数
			return extractLatencyMs(timePart)
		}

		// 检查 "时间<" 格式（响应时间小于1ms）
		if idx := strings.Index(line, "时间<"); idx >= 0 {
			// 时间<1ms 表示延迟小于1ms，返回0
			return 0
		}

		// === Windows/Linux 英文格式 ===
		// Windows: "Reply from 192.168.1.1: bytes=32 time=14ms TTL=64"
		// Windows快速: "Reply from 192.168.1.1: bytes=32 time<1ms TTL=64"
		// Linux: "64 bytes from 192.168.1.1: icmp_seq=1 ttl=64 time=1.0 ms"

		// 检查 "time=" 格式
		if idx := strings.Index(line, "time="); idx >= 0 {
			timePart := line[idx+len("time="):]
			return extractLatencyMs(timePart)
		}

		// 检查 "time<" 格式
		if idx := strings.Index(line, "time<"); idx >= 0 {
			return 0
		}
	}

	return 0
}

// extractLatencyMs 从 "14ms TTL=64" 或 "1.0 ms" 这类字符串中提取延迟毫秒数
func extractLatencyMs(timePart string) int {
	// 去除前导空格
	timePart = strings.TrimSpace(timePart)

	// 找到数字结束的位置
	numStr := ""
	hasDecimal := false
	for _, ch := range timePart {
		if ch >= '0' && ch <= '9' {
			numStr += string(ch)
		} else if ch == '.' && !hasDecimal {
			hasDecimal = true
			numStr += string(ch)
		} else {
			break
		}
	}

	if numStr == "" {
		return 0
	}

	if hasDecimal {
		var latency float64
		fmt.Sscanf(numStr, "%f", &latency)
		return int(latency)
	}

	var latency int
	fmt.Sscanf(numStr, "%d", &latency)
	return latency
}

// getMACAddress 获取IP对应的MAC地址
func getMACAddress(ip string) string {
	switch runtime.GOOS {
	case "windows":
		return getMACAddressWindows(ip)
	case "linux", "darwin":
		return getMACAddressLinux(ip)
	default:
		return ""
	}
}

// getMACAddressWindows Windows平台获取MAC地址
func getMACAddressWindows(ip string) string {
	cmd := exec.Command("arp", "-a", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Windows ARP表格式：
	// 接口: 192.168.1.100 --- 0x2
	//   Internet 地址         物理地址              类型
	//   192.168.1.1           aa-bb-cc-dd-ee-ff     动态

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ip) {
			// 提取MAC地址
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mac := fields[1]
				if isValidMAC(mac) {
					return mac
				}
			}
		}
	}
	return ""
}

// getMACAddressLinux Linux/Mac平台获取MAC地址
func getMACAddressLinux(ip string) string {
	cmd := exec.Command("arp", "-n", ip)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Linux ARP表格式：
	// Address                  HWtype  HWaddress           Flags Mask            Iface
	// 192.168.1.1              ether   aa:bb:cc:dd:ee:ff   C                     eth0

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ip) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				mac := fields[2]
				if isValidMAC(mac) {
					return mac
				}
			}
		}
	}
	return ""
}

// isValidMAC 检查MAC地址是否有效
func isValidMAC(mac string) bool {
	if mac == "" || mac == "(incomplete)" || mac == "<incomplete>" {
		return false
	}
	// 检查是否包含足够的字符（MAC地址通常是17个字符，如"aa-bb-cc-dd-ee-ff"或"aa:bb:cc:dd:ee:ff"）
	return len(mac) >= 14
}

// generateIPList 生成起始和结束IP之间的所有IP地址
func generateIPList(startIP, endIP net.IP) ([]string, error) {
	var ips []string

	start := ipToUint32(startIP.To4())
	end := ipToUint32(endIP.To4())

	if start > end {
		return nil, fmt.Errorf("起始IP不能大于结束IP")
	}

	for i := start; i <= end; i++ {
		ip := uint32ToIP(i)
		ips = append(ips, ip.String())
	}

	return ips, nil
}

// ipToUint32 将IPv4转换为uint32
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP 将uint32转换为IPv4
func uint32ToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte((n >> 24) & 0xFF)
	ip[1] = byte((n >> 16) & 0xFF)
	ip[2] = byte((n >> 8) & 0xFF)
	ip[3] = byte(n & 0xFF)
	return ip
}
