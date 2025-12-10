package internal

import (
	"GoWork_4/tools"
	"fmt"
	"log"
	"net"
	"strings"
)

// handleLogin 处理客户端登录过程
// 参数 conn 是客户端的网络连接
// 返回值表示是否成功完成登录流程
func (s *Server) handleLogin(conn net.Conn) bool {
	name := ""
	var i int
	for {
		err := tools.SendMessage(conn, "请输入昵称：")
		if err != nil {
			return false
		}
		nameInput, err := tools.ReceiveMessage(conn)
		if err != nil {
			return true // 客户端断开
		}

		nameInput = strings.TrimSpace(nameInput)

		// 1. 昵称格式验证
		if valid, reason := ValidateName(nameInput); !valid {
			err := tools.SendMessage(conn, fmt.Sprintf("昵称无效: %s，请重新输入昵称：", reason))
			if err != nil {
				return false
			}
			continue
		}

		// 2. 在线状态检查
		if s.isNameTaken(nameInput) {
			err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 已在线，请重新输入昵称：", nameInput))
			if err != nil {
				return false
			}
			continue
		}

		// 3. 数据库注册状态检查
		isRegistered, err := s.userDB.CheckNameExists(nameInput)
		if err != nil {
			fmt.Printf("[DB 错误] 检查用户名 '%s' 失败: %v\n", nameInput, err)
			err := tools.SendMessage(conn, "服务器数据库错误，请稍后再试。")
			if err != nil {
				return false
			}
			return true // 数据库错误，断开连接
		}

		if !isRegistered {
			// 昵称未注册，要求用户返回主菜单选择注册
			err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 未注册，请返回主菜单选择注册。", nameInput))
			if err != nil {
				return false
			}
			return false // 返回 false，回到主循环选择菜单
		}

		name = nameInput
		break // 昵称校验通过，进入密码输入
	}

	err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 已注册，请输入密码：", name))
	if err != nil {
		return false
	}

	for {
		password, err := tools.ReceiveMessage(conn)
		if err != nil {
			fmt.Printf("有问题")
			return true
		}
		if password == "" {
			err := tools.SendMessage(conn, "密码不能为空，请重新输入密码：")
			if err != nil {
				return false
			}
			continue
		}
		success, err := s.userDB.CheckCredentials(name, password)
		if success && err == nil {
			// 登录成功
			s.registerClient(conn, name)
			err := tools.SendMessage(conn, fmt.Sprintf("欢迎 %s！您已成功登录，开始聊天吧...\n使用 /help 查看可用命令", name))
			if err != nil {
				return false
			}
			s.broadcastChan <- &ClientMessage{
				Conn:    conn,
				Name:    name,
				Message: fmt.Sprintf("系统: %s 加入了聊天室", name),
				Type:    "system", // 标记为系统消息
			}
			s.handleClientChat(conn, name)
			return true // 登录成功，退出注册函数
		}
		failReason := "登录失败，密码不正确"
		if err != nil {
			fmt.Printf("[DB 错误] 登录验证失败: %v\n", err)
			failReason = "登录失败：数据库验证错误，即将断开连接。"
			err := tools.SendMessage(conn, failReason)
			if err != nil {
				return false
			}
			return true // 数据库错误，断开连接
		}
		err = tools.SendMessage(conn, failReason)
		if err != nil {
			return false
		}
		if i < 2 {
			i++
			err := tools.SendMessage(conn, "请重新输入密码：")
			if err != nil {
				return false
			}
			continue
		} else {
			break
		}
	}
	// 3 次密码输入失败
	err = tools.SendMessage(conn, "密码输入错误次数过多，请重新输入昵称。")
	if err != nil {
		return false
	}
	return false // 返回 false，回到外层循环重新输入昵称
}

// handleRegistrationLogic 处理客户端注册流程，包括昵称校验和重复检查
// 参数 conn 是客户端的网络连接
// 返回值表示是否成功完成注册流程
func (s *Server) handleRegistrationLogic(conn net.Conn) bool {
	name := ""

	for {
		err := tools.SendMessage(conn, "请输入您想要注册的昵称：")
		if err != nil {
			return false
		}
		nameInput, err := tools.ReceiveMessage(conn)
		if err != nil {
			return true // 客户端断开
		}

		nameInput = strings.TrimSpace(nameInput)

		// 1. 昵称格式验证 (假设存在 ValidateName 函数)
		if valid, reason := ValidateName(nameInput); !valid {
			err := tools.SendMessage(conn, fmt.Sprintf("昵称无效: %s，请重新输入昵称：", reason))
			if err != nil {
				return false
			}
			continue
		}

		// 2. 在线状态检查
		if s.isNameTaken(nameInput) {
			err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 已在线，请重新输入昵称：", nameInput))
			if err != nil {
				return false
			}
			continue
		}

		// 3. 数据库注册状态检查
		isRegistered, err := s.userDB.CheckNameExists(nameInput)
		if err != nil {
			fmt.Printf("[DB 错误] 检查用户名 '%s' 失败: %v\n", nameInput, err)
			err := tools.SendMessage(conn, "服务器数据库错误，请稍后再试。")
			if err != nil {
				return false
			}
			return true // 数据库错误，断开连接
		}

		if isRegistered {
			// 昵称已注册，引导用户返回主菜单选择登录
			err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 已注册，请返回主菜单选择登录。", nameInput))
			if err != nil {
				log.Printf("注册失败了handleBroadcasts")
				return false
			}
			return false // 返回 false，回到主循环选择菜单
		}

		name = nameInput
		break // 昵称校验通过，进入密码输入
	}

	err := tools.SendMessage(conn, fmt.Sprintf("昵称 '%s' 可用。请输入密码进行注册：", name))
	if err != nil {
		return false
	}

	password, err := tools.ReceiveMessage(conn)
	if err != nil {
		return true // 客户端断开，退出
	}

	if password == "" {
		tools.SendMessage(conn, "密码不能为空，请重新输入昵称：")
		return false // 返回 false，回到外层循环重新输入昵称
	}

	err = s.userDB.RegisterUser(name, password)
	if err != nil {
		// 注册失败
		fmt.Printf("[DB 错误] 注册失败: %v\n", err)
		tools.SendMessage(conn, "注册失败：数据库写入错误。请重新输入昵称：")
		return false // 返回 false，回到外层循环重新输入昵称
	}
	tools.SendMessage(conn, fmt.Sprintf("恭喜 %s 注册成功！请返回主菜单。", name))

	return false // 注册成功，退出注册函数
}

// handleClientChat 负责接收并转发客户端发送的消息，支持命令解析和私聊功能
// 参数 conn 是客户端的网络连接，name 是该用户的昵称
func (s *Server) handleClientChat(conn net.Conn, name string) {
	defer func() {
		s.unregisterChan <- conn
	}()

	for {
		// 接收消息
		input, err := tools.ReceiveMessage(conn)
		if err != nil {
			// 客户端断开连接或读取失败，退出循环，执行 defer
			break
		}

		s.handleClientChatAndCommand(conn, name, input) // <--- 调用我们之前设计的统一处理函数}
	}
}

func (s *Server) handleClientChatAndCommand(conn net.Conn, name, input string) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "/") {
		if s.handleCommand(conn, input) {
			return
		}
	}
	if strings.HasPrefix(input, "@") {
		// 私聊处理逻辑（应将 Type:"private" 的消息发送到 s.messageChan）
		s.handlePrivateMessage(conn, name, input) // <--- 确保此函数内部只发消息到 s.messageChan
		return
	}
	s.handleChatMessage(conn, name, input)
}

func (s *Server) handleChatMessage(conn net.Conn, name, message string) {
	// 关键：只将消息发送到 messageChan，将 Type 设置为 "chat"
	chatMsg := &ClientMessage{
		Conn:    conn,
		Name:    name,
		Message: message,
		Type:    "chat",
		Target:  "",
	}
	s.messageChan <- chatMsg
}

// handlePrivateMessage 处理私聊消息
// 参数 conn 是发送方的连接，sender 是发送方昵称，message 是原始消息文本
func (s *Server) handlePrivateMessage(conn net.Conn, sender string, message string) {
	// 1. 解析私聊消息格式: @用户名 消息内容
	// message 应该形如 "@targetName content"
	parts := strings.SplitN(message, " ", 2)

	// 如果 parts[0] 是 @targetName，需要移除 @ 符号
	targetNameWithAt := strings.TrimSpace(parts[0])
	targetName := strings.TrimPrefix(targetNameWithAt, "@")

	if len(parts) < 2 || targetName == "" {
		tools.SendMessage(conn, "【系统】私聊格式错误，请使用: @用户名 消息内容")
		return
	}

	content := parts[1]

	if targetName == sender {
		tools.SendMessage(conn, "【系统】不能给自己发送私聊消息")
		return
	}

	// 2. 查找目标用户（在新架构中，我们只检查**目标是否在线**，实际发送交给 broadcastMessage）
	if _, exists := s.getClientConnection(targetName); !exists {
		tools.SendMessage(conn, fmt.Sprintf("【系统】用户 '%s' 不在线或不存在", targetName))
		return
	}

	// 3. 封装为 ClientMessage 并发送到 messageChan
	// Type 设置为 "private"，Target 设置为目标用户名
	privateMsg := &ClientMessage{
		Conn:    conn,       // 保持原始连接，用于给发送者确认
		Name:    sender,     // 发送者
		Message: content,    // 消息内容
		Type:    "private",  // 关键：私聊消息类型
		Target:  targetName, // 关键：目标用户
	}

	// 将私聊消息交给中心消息处理协程 (handleMessages -> handleBroadcasts)
	s.messageChan <- privateMsg
	// `server_core.go` 中的 `broadcastMessage` 函数来发送，以保持一致性。
}

// handleCommand 解析并执行客户端发送的命令
// 参数 conn 是客户端连接，message 是命令文本
// 返回布尔值表示是否是有效命令
func (s *Server) handleCommand(conn net.Conn, message string) bool {
	if len(message) > 0 && message[0] == '/' {
		parts := strings.Fields(message)
		if len(parts) == 0 {
			return true
		}

		command := parts[0]
		switch command {
		case "/list":
			tools.SendMessage(conn, s.getOnlineUsers())
		case "/help":
			helpMsg := `可用命令：
/list - 查看在线用户
/help - 显示帮助信息
/history或/h - 查看最近的10条历史消息
/rank - 查看活跃度排名前五的用户
/exit - 退出
私聊功能：
@用户名 消息内容 - 发送私聊消息
例如: @张三 你好！`
			tools.SendMessage(conn, helpMsg)
		case "/history", "/h":
			const defaultHistoryCount = 10
			if s.asyncQueue == nil || s.asyncQueue.Client == nil {
				tools.SendMessage(conn, "系统：历史记录功能当前不可用(Redis未连接)")
				break
			}
			history, err := s.asyncQueue.GetChatHistory(defaultHistoryCount)
			if err != nil {
				tools.SendMessage(conn, fmt.Sprintf("系统：获取历史记录"))
			}
			if len(history) == 0 {
				tools.SendMessage(conn, "系统,暂无聊天历史记录")
			} else {
				msg := fmt.Sprintf("--- 最近 %d 条聊天历史记录 ---\n%s\n--- 历史记录结束 ---", len(history), strings.Join(history, "\n"))
				tools.SendMessage(conn, msg)
			}
		case "/rank":
			const defaultRankCount = 5
			if s.asyncQueue == nil || s.asyncQueue.Client == nil {
				tools.SendMessage(conn, "系统：活跃度排名功能当前不可用（Redis未连接）")
			}
			rankList, err := s.asyncQueue.GetActivityRank(defaultRankCount)
			if err != nil {
				tools.SendMessage(conn, fmt.Sprintf("系统：获取活跃度排名失败：%v", err))
				break
			}
			if len(rankList) == 0 {
				tools.SendMessage(conn, "系统：暂无活跃度数据。")
			} else {
				msg := fmt.Sprintf("--- 活跃度排名前 %d 用户 ---\n%s\n--- 排名结束 ---", len(rankList), strings.Join(rankList, "\n"))
				tools.SendMessage(conn, msg)
			}

		default:
			tools.SendMessage(conn, "未知命令，使用 /help 查看可用命令")
		}
		return true
	}
	return false
}

// getClientConnection 获取指定用户名对应的客户端连接
// 参数 name 是目标用户名
// 返回值 conn 是对应连接，exists 标识是否存在
func (s *Server) getClientConnection(name string) (net.Conn, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	conn, exists := s.clients[name]
	return conn, exists
}

// isNameTaken 判断某个用户名是否已被占用
// 参数 name 是待检测的用户名
// 返回布尔值表示是否已被占用
func (s *Server) isNameTaken(name string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, exists := s.clients[name]
	return exists
}

// registerClient 将新客户端注册进服务器内部数据结构中
// 参数 conn 是客户端连接，name 是其昵称
func (s *Server) registerClient(conn net.Conn, name string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.clients[name] = conn
	s.clientConnToName[conn] = name

	fmt.Printf("客户端注册成功: %s (%s)\n", name, conn.RemoteAddr())

	// 发送用户上线系统消息
	joinMsg := fmt.Sprintf("【系统消息】用户 %s 上线了！当前在线人数: %d", name, len(s.clients))
	systemMsg := &ClientMessage{
		Name:    "[系统]",
		Message: joinMsg,
		Type:    "system", // 标记为系统消息
		Conn:    nil,
	}
	s.messageChan <- systemMsg

	fmt.Printf("当前在线用户: %d\n", len(s.clients))
}

// removeClient 从服务器移除指定客户端连接及其相关信息
// 参数 conn 是需要移除的客户端连接
func (s *Server) removeClient(conn net.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if name, exists := s.clientConnToName[conn]; exists {
		if currentConn, userExists := s.clients[name]; userExists && currentConn == conn {
			// ... existing logic to delete from maps
			delete(s.clients, name)          // s.clients 长度变为 0
			delete(s.clientConnToName, conn) // s.clientConnToName 长度变为 0
			currentOnline := len(s.clients)  // currentOnline = 0 (准确)
			// **新增/修改：发送用户下线系统消息**
			leaveMsg := fmt.Sprintf("【系统消息】用户 %s 离开了！当前在线人数: %d", name, len(s.clients)-1)
			systemMsg := &ClientMessage{
				Name:    "[系统]",
				Message: leaveMsg,
				Type:    "system", // 标记为系统消息
				Conn:    nil,
			}
			s.messageChan <- systemMsg

			fmt.Printf("客户端移除成功: %s (%s)\n", name, conn.RemoteAddr())
			fmt.Printf("当前在线用户: %d\n", currentOnline)
			conn.Close()
		}
	}
}

// getOnlineUsers 获取当前在线的所有用户名列表
// 返回格式化后的字符串显示在线用户数量及名单
func (s *Server) getOnlineUsers() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if len(s.clients) == 0 {
		return "在线用户: 无"
	}

	var users []string
	for name := range s.clients {
		users = append(users, name)
	}

	return fmt.Sprintf("在线用户 (%d): %s", len(users), strings.Join(users, ", "))
}
