package internal

import (
	"GoWork_4/chat_server/db"
	"GoWork_4/chat_server/rdb"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
)

// ClientMessage 客户端消息结构
// 用于封装客户端发送的消息信息，包括连接、名称、消息内容等字段。
type ClientMessage struct {
	Conn    net.Conn // 客户端网络连接
	Name    string   // 用户名
	Message string   // 消息内容
	Type    string   // 消息类型（如 chat/system）
	Target  string   // 私聊目标用户
}

// Server 服务器结构
// 包含所有客户端连接管理、消息处理通道及同步控制组件。
type Server struct {
	clients          map[string]net.Conn // 存储用户名到连接的映射
	clientConnToName map[net.Conn]string // 存储连接到用户名的映射
	mutex            sync.RWMutex        // 读写锁保护并发访问
	messageChan      chan *ClientMessage // 接收普通消息的通道
	broadcastChan    chan *ClientMessage // 广播消息通道
	registerChan     chan net.Conn       // 注册新客户端连接的通道
	unregisterChan   chan net.Conn       // 取消注册客户端连接的通道
	Done             chan struct{}       // 控制服务停止的信号通道
	userDB           *db.UserDB
	asyncQueue       *rdb.RedisQueueClient
}

// NewServer 创建一个新的服务器实例并初始化相关字段
// 返回一个指向 Server 的指针
func NewServer() *Server {

	redisAddr := "localhost:6379"
	redisPassword := ""
	redisDB := 2
	rdb.NewRedisQueueClient(redisAddr, redisPassword, redisDB)
	s := &Server{
		clients:          make(map[string]net.Conn),
		clientConnToName: make(map[net.Conn]string),
		messageChan:      make(chan *ClientMessage, 100),
		broadcastChan:    make(chan *ClientMessage, 100),
		registerChan:     make(chan net.Conn, 10),
		unregisterChan:   make(chan net.Conn, 10),
		Done:             make(chan struct{}),
	}
	s.userDB = db.ConnectDB()
	s.asyncQueue = rdb.NewRedisQueueClient(redisAddr, redisPassword, redisDB)
	if s.asyncQueue != nil && s.asyncQueue.Client != nil {
		ctx := context.Background()

		// 删除 Stream Key
		if err := s.asyncQueue.Client.Del(ctx, rdb.ChatStreamKey).Err(); err != nil {
			fmt.Printf("警告：启动时清空 Redis Stream 失败: %v\n", err)
		}

		// 删除 Rank Key
		if err := s.asyncQueue.Client.Del(ctx, rdb.ChatRankKey).Err(); err != nil {
			fmt.Printf("警告：启动时清空 Redis Rank Set 失败: %v\n", err)
		}
	}

	return s
}

// ValidateName 验证用户名是否合法
// 参数 name 表示待验证的用户名字符串
// 返回值 valid 表示验证结果，reason 提供错误原因描述
func ValidateName(name string) (bool, string) {
	if len(name) == 0 {
		return false, "昵称不能为空"
	}

	if len(name) > 20 {
		return false, "昵称长度不能超过20个字符"
	}

	for _, char := range name {
		if char < 32 || char > 126 {
			return false, "昵称包含非法字符"
		}
		if strings.ContainsRune("\\/:*?\"<>|", char) {
			return false, "昵称包含不允许的特殊字符"
		}
	}

	return true, ""
}
