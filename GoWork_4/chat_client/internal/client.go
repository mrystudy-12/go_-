package internal

import (
	"fmt"
	"net"
	"sync/atomic"
)

// Client 表示一个聊天客户端，用于与服务器通信。
type Client struct {
	conn        net.Conn      // 客户端到服务器的网络连接
	name        string        // 用户昵称
	sendChan    chan string   // 发送消息通道（缓冲大小为10）
	receiveChan chan string   // 接收消息通道（缓冲大小为10）
	errorChan   chan error    // 错误信息通道（缓冲大小为1）
	done        chan struct{} // 通知所有goroutine退出的信号通道
	isConnected int32         // 原子变量表示是否处于连接状态（1=连接中，0=未连接）
}

// NewClient 创建一个新的客户端实例，并初始化相关字段。
// 返回值：指向新创建的Client结构体指针
func NewClient() *Client {
	return &Client{
		sendChan:    make(chan string, 10),
		receiveChan: make(chan string, 10),
		errorChan:   make(chan error, 1),
		done:        make(chan struct{}),
		isConnected: 1,
	}
}

// Connect 尝试建立到指定地址的TCP连接。
// 参数 addr 是目标服务器地址，格式如 "host:port"
// 返回值：如果连接失败则返回错误；否则返回nil
func (c *Client) Connect(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	c.conn = conn
	atomic.StoreInt32(&c.isConnected, 1)
	return nil
}

// Start 启动客户端的主要逻辑流程。
// 包括用户认证、启动多个后台协程处理收发消息等操作。
func (c *Client) Start() {
	defer c.safeRecover("客户端主循环")

	if err := c.handleAuthentication(); err != nil { // handleAuthentication 在 client_auth.go 中
		fmt.Printf("注册/登录失败: %v\n", err)
		return
	}

	go c.safeReceiveFromServer() // safeReceiveFromServer 在 client_io.go 中
	go c.safeSendToServer()      // safeSendToServer 在 client_io.go 中
	go c.safeHandleMessages()    // safeHandleMessages 在 client_io.go 中

	c.userInputLoop() // userInputLoop 在 client_io.go 中
}

// handleConnectionError 负责统一处理连接相关的错误。
// 打印错误日志并执行资源清理工作。
func (c *Client) handleConnectionError(err error) {
	fmt.Printf("\r连接异常: %v\n", err)
	c.cleanup()
}

// safeRecover 捕获可能发生的panic并记录上下文信息。
// 最终调用cleanup释放资源。
func (c *Client) safeRecover(context string) {
	if r := recover(); r != nil {
		fmt.Printf("%s: %v\n", context, r)
	}
	c.cleanup()
}

// cleanup 清理客户端占用的所有资源。
// 关闭网络连接、关闭done通道并标记为未连接状态。
func (c *Client) cleanup() {
	if atomic.CompareAndSwapInt32(&c.isConnected, 1, 0) {
		fmt.Println("正在清理资源...")

		if c.conn != nil {
			c.conn.Close()
		}

		select {
		case <-c.done:
		default:
			close(c.done)
		}

		fmt.Println("资源清理完成")
	}
}

// isConnectedAtomic 判断当前客户端是否仍处于连接状态。
// 返回true表示仍在连接中，false表示已经断开。
func (c *Client) isConnectedAtomic() bool {
	//
	return atomic.LoadInt32(&c.isConnected) == 1
}
