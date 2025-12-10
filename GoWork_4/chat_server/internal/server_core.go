package internal

import (
	"GoWork_4/chat_server/rdb"
	"GoWork_4/tools"
	"fmt"
	"net"
	"strings"
	"time"
)

func (s *Server) ChatTaskHandler(msg *rdb.ChatMessage) {
	// 将从 Redis 消费到的消息结构体转换为服务器内部的 ClientMessage 结构体
	clientMsg := &ClientMessage{
		Name:    msg.Name,
		Message: msg.Message,
		Type:    msg.Type,
		Conn:    nil, // 消费者处理的消息不需要原始连接
	}

	// 放入广播通道，由 handleBroadcasts 协程进行统一广播
	s.broadcastChan <- clientMsg
}

// Start 启动 TCP 服务器监听指定端口，并开启多个协程处理不同任务
// 参数 port 是要监听的端口号字符串
func (s *Server) Start(port string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Printf("服务器启动失败: %v\n", err)
		return
	}
	defer listener.Close()
	go s.handleMessages()
	go s.handleBroadcasts()
	go s.acceptConnections(listener)
	const asyncChatConsumerCount = 3
	if s.asyncQueue != nil {
		// 检查并创建消费者组
		if err := s.asyncQueue.CreateChatConsumerGroup(); err != nil {
			fmt.Printf("警告：创建 Redis Stream 消费者组失败: %v\n", err)
		}

		// 2. 启动 3 个消费者
		for i := 1; i <= asyncChatConsumerCount; i++ {
			consumerName := fmt.Sprintf("chat-consumer-%d", i)
			// 启动消费者协程，传入 ChatTaskHandler 作为回调函数
			s.asyncQueue.StartChatConsumer(consumerName, s.ChatTaskHandler)
		}

	} else {
		fmt.Println("警告：Redis异步队列未连接或初始化失败，异步任务功能将不可用")
	}

	<-s.Done
}

// acceptConnections 接受新的客户端连接请求并将连接加入注册队列
// 参数 listener 是已经建立好的监听器对象
func (s *Server) acceptConnections(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.Done:
				return
			default:
				fmt.Printf("接受连接失败: %v\n", err)
				time.Sleep(10 * time.Millisecond) // 避免忙等待
				continue
			}
		}
		s.registerChan <- conn
	}
}

// handleMessages 处理客户端连接注册与注销事件
func (s *Server) handleMessages() {
	for {
		select {
		case <-s.Done:
			return
		case msg := <-s.messageChan:
			if msg == nil {
				continue
			}
			if msg.Type == "system" || msg.Type == "private" {
				s.broadcastChan <- msg
			} else {
				// 普通聊天消息 (msg.Type == "chat")

				// 1. 活跃度增加（同步操作，放在入队前） 🌟 新增活跃度逻辑
				if s.asyncQueue != nil {
					if err := s.asyncQueue.IncrUserAction(msg.Name); err != nil {
						fmt.Printf("警告: 增加用户活跃度失败：%v\n", err)
					}
				}

				// 2. 异步发送到 Redis Stream，由消费者组处理
				chatMsg := &rdb.ChatMessage{
					Name:    msg.Name,
					Message: msg.Message,
					Type:    msg.Type,
				}
				if err := s.asyncQueue.AsyncProduceMessage(chatMsg); err != nil {
					fmt.Printf("警告：消息异步入队失败: %v，将尝试同步广播。\n", err)
					// 入队失败回退：立即广播
					s.broadcastChan <- msg
				}
			}
		case conn := <-s.registerChan:
			go s.handleAuthentication(conn)
		case conn := <-s.unregisterChan:
			s.removeClient(conn)
		}
	}
}

// handleAuthentication 处理客户端认证流程，包括登录和注册的选择
// 参数 conn 是客户端的网络连接
func (s *Server) handleAuthentication(conn net.Conn) {
	defer func() {
		s.mutex.RLock()
		_, exists := s.clientConnToName[conn]
		s.mutex.RUnlock()
		if !exists {
			conn.Close()
		}
	}()

	for {
		tools.SendMessage(conn, "欢迎！请选择操作：\n1.登录\n2.注册")
		selection, err := tools.ReceiveMessage(conn)
		if err != nil {
			return
		}
		switch strings.TrimSpace(selection) {
		case "1":
			if s.handleLogin(conn) {
				return
			}
		case "2":
			if s.handleRegistrationLogic(conn) {
				return
			}
		default:
			tools.SendMessage(conn, "无效的选项，请重新输入：")
		}
	}
}

// handleBroadcasts 监听广播消息通道并将消息分发给所有在线客户端
func (s *Server) handleBroadcasts() {
	for {
		select {
		case <-s.Done:
			return
		case msg := <-s.broadcastChan:
			s.broadcastMessage(msg)
		}
	}
}

// broadcastMessage 实际将消息广播至所有在线客户端（包括私聊定向发送和连接清理）
// 参数 clientMsg 是待广播的消息体
func (s *Server) broadcastMessage(clientMsg *ClientMessage) {
	s.mutex.RLock()

	var connsToCleanup []net.Conn

	switch clientMsg.Type {
	case "system":
		broadcastMsg := clientMsg.Message

		// 广播给所有客户端
		for name, conn := range s.clients {
			err := tools.SendMessage(conn, broadcastMsg)
			if err != nil {
				fmt.Printf("发送系统消息给 %s 失败，标记清理: %v\n", name, err)
				connsToCleanup = append(connsToCleanup, conn)
			}
		}

	case "private":
		// 1. 发送给目标用户 (Target)
		targetConn, exists := s.clients[clientMsg.Target]
		if exists {
			// [私聊 - 张三 悄悄对你说]: 你好
			msgToTarget := fmt.Sprintf("【私聊 - %s】: %s", clientMsg.Name, clientMsg.Message)
			if err := tools.SendMessage(targetConn, msgToTarget); err != nil {
				fmt.Printf("发送私聊消息给目标用户 %s 失败，标记清理: %v\n", clientMsg.Target, err)
				connsToCleanup = append(connsToCleanup, targetConn)
			}
		} else {
			// 在 handleMessages 中已经做了初步检查，但这里是最终发送点。如果目标突然离线，会在这里失效。
			// 如果在 handleMessages 之前检查，此处可以省略，但为健壮性保留。
		}

		// 2. 发送确认给发送者 (Name)
		senderConn, senderExists := s.clients[clientMsg.Name]
		if senderExists {
			// [私聊 - 你悄悄对 李四 说]: 你好
			msgToSender := fmt.Sprintf("【私聊%s】: %s", clientMsg.Target, clientMsg.Message)
			if err := tools.SendMessage(senderConn, msgToSender); err != nil {
				fmt.Printf("发送私聊确认消息给发送者 %s 失败，标记清理: %v\n", clientMsg.Name, err)
				connsToCleanup = append(connsToCleanup, senderConn)
			}
		}

	case "chat": // 普通聊天消息（可能来自同步的 handleMessages 失败回退，或来自异步的 ChatTaskHandler）
		broadcastMsg := fmt.Sprintf("[%s]: %s", clientMsg.Name, clientMsg.Message)

		// 广播给所有客户端
		for name, conn := range s.clients {
			err := tools.SendMessage(conn, broadcastMsg)
			if err != nil {
				fmt.Printf("发送聊天消息给 %s 失败，标记清理: %v\n", name, err)
				connsToCleanup = append(connsToCleanup, conn)
			}
		}

	default:
		// 忽略未知类型消息
		return
	}

	s.mutex.RUnlock() // 🌟 释放读锁

	// --- 连接清理逻辑 ---
	// 遍历收集到的失效连接列表，将它们送入注销通道进行异步清理。
	for _, conn := range connsToCleanup {
		// 检查连接是否已经在清理队列中（避免重复操作）
		// 由于 unregisterChan 是有缓冲的，这里是安全的。

		// 1. 将连接放入注销队列，触发 removeClient() 协程（在 handleConnections 中）
		s.unregisterChan <- conn
	}
}
func (s *Server) Stop() {
	fmt.Println("正在关闭服务器...")
	select {
	case <-s.Done:
		// done 通道已关闭，不需要重复操作
	default:
		// 关闭 done 通道，解除 main goroutine 的阻塞
		close(s.Done)
	}
	if s.userDB != nil {
		s.userDB.Close()
	}
	if s.asyncQueue != nil && s.asyncQueue.Client != nil {
		s.asyncQueue.Client.Close()
		fmt.Println("Redis 异步队列连接已关闭。")
	}
	s.mutex.Lock()
	for name, conn := range s.clients {
		tools.SendMessage(conn, "系统: 服务器正在关闭，连接即将断开")
		conn.Close()
		fmt.Printf("已断开: %s\n", name)
	}
	s.clients = make(map[string]net.Conn)
	s.clientConnToName = make(map[net.Conn]string)
	s.mutex.Unlock()

	fmt.Println("服务器已关闭")
}
