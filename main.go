package main

import (
	"fmt"
	"log"
	"net/http"
	"netscanner/api"
)

func main() {
	// 注册API处理函数
	http.HandleFunc("/api/scan", api.ScanHandler)

	// 提供静态文件服务
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// 启动HTTP服务器
	port := ":8080"
	fmt.Printf("服务器启动在 http://localhost%s\n", port)
	fmt.Println("按 Ctrl+C 停止服务器")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("服务器启动失败: ", err)
	}
}
