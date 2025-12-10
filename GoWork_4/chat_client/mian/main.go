package main

import (
	"GoWork_4/chat_client/internal"
	"fmt"
)

// main 入口函数。负责创建客户端对象并尝试连接服务器。
// 成功连接后启动客户端主逻辑。
func main() {
	client := internal.NewClient()

	//fmt.Println("正在连接聊天服务器...")
	if err := client.Connect("localhost:15000"); err != nil {
		fmt.Printf("连接失败: %v\n", err)
		fmt.Println("请确保服务器已启动并监听端口 15000")
		return
	}

	//fmt.Println("连接成功！")
	client.Start()

	fmt.Println("按回车键退出...")
	fmt.Scanln()
}
