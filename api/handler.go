package api

import (
	"encoding/json"
	"net/http"
	"time"

	"netscanner/scanner"
)

// ScanRequest 扫描请求
type ScanRequest struct {
	StartIP string `json:"startIP"`
	EndIP   string `json:"endIP"`
}

// ScanResponse 扫描响应
type ScanResponse struct {
	Success bool                 `json:"success"`
	Message string               `json:"message"`
	Results []scanner.ScanResult `json:"results"`
}

// ScanHandler 处理扫描请求
func ScanHandler(w http.ResponseWriter, r *http.Request) {
	// 只处理POST请求
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "无效的请求格式", http.StatusBadRequest)
		return
	}

	// 验证IP地址
	if req.StartIP == "" || req.EndIP == "" {
		sendErrorResponse(w, "请提供起始IP和结束IP", http.StatusBadRequest)
		return
	}

	// 执行扫描（使用改进的混合检测方案：ICMP Ping + ARP检查）
	opts := scanner.ScanOptions{
		StartIP:    req.StartIP,
		EndIP:      req.EndIP,
		Timeout:    1 * time.Second, // 缩短超时时间以提高速度
		Concurrent: 100,             // 增加并发数以提高扫描速度
		UseARP:     true,            // 启用ARP检查
		UseICMP:    true,            // 启用ICMP Ping（主要检测方法）
	}

	results, err := scanner.Scan(opts)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回成功响应
	response := ScanResponse{
		Success: true,
		Message: "扫描完成",
		Results: results,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse 发送错误响应
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ScanResponse{
		Success: false,
		Message: message,
		Results: nil,
	})
}
