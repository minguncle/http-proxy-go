package main

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
)

// proxyHandler 根据请求类型选择处理方式
func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		handleTunneling(w, r) // 处理 HTTPS 隧道请求
	} else {
		handleHTTP(w, r) // 处理 HTTP 请求
	}
}

// handleTunneling 处理 HTTPS 隧道请求
func handleTunneling(w http.ResponseWriter, r *http.Request) {
	destConn, err := net.Dial("tcp", r.Host) // 连接目标服务器
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK) // 返回成功状态码
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack() // 获取客户端连接
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	go transfer(destConn, clientConn) // 开启协程进行数据转发
	go transfer(clientConn, destConn) // 开启协程进行数据转发
}

// handleHTTP 处理 HTTP 请求
func handleHTTP(w http.ResponseWriter, r *http.Request) {
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http" // 设置请求协议
			req.URL.Host = r.Host   // 设置请求主机
		},
	}
	rp.ServeHTTP(w, r) // 反向代理处理请求
}

// transfer 负责在两个连接之间传输数据
func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()    // 关闭目标连接
	defer source.Close()         // 关闭源连接
	io.Copy(destination, source) // 数据拷贝
}

func main() {
	server := &http.Server{
		Addr:         ":8006",                                                      // 监听地址
		Handler:      http.HandlerFunc(proxyHandler),                               // 设置处理函数
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)), // 禁用 HTTP/2
	}

	log.Printf("HTTP proxy server is running on %s", server.Addr) // 输出监听地址
	if err := server.ListenAndServe(); err != nil {               // 启动服务器
		log.Fatalf("Failed to start the server: %v", err) // 输出错误信息
	}
}
