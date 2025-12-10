package main

import (
	"GoWork_4/chat_server/internal"
	"fmt"
)

// main 主程序入口，创建服务器实例并启动监听，同时提供手动关闭机制
func main() {
	server := internal.NewServer()

	go server.Start("15000")
	fmt.Println("服务器已启动，等待外部信号关闭...")

	// 阻塞主 goroutine，直到服务器的 done 通道被关闭
	<-server.Done

	fmt.Println("服务器已关闭。")
}
