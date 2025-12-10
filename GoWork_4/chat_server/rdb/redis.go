package rdb

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"strings"
	"time"
)

// RedisQueueClient 是一个基于 Redis 的队列客户端结构体，
// 提供消息入队和消费功能。
type RedisQueueClient struct {
	Client    *redis.Client // Redis 客户端实例
	QueueKey  string        // 队列在 Redis 中对应的键名
	StreamKey string        // Stream 键名，用于存储日志消息
	GroupKey  string        // 消费者组键名
}

const (
	// ChatStreamKey 聊天历史记录流的键名，用于存储所有聊天消息的历史记录
	ChatStreamKey = "chat_history_stream"

	// TaskStreamKey 服务器任务流键名，用于存储需要异步处理的服务器任务
	TaskStreamKey = "server_task_stream"

	// TaskGroupKey 任务消费者组键名，用于Redis Stream的消费者组标识
	TaskGroupKey = "task_consumer_group"

	// ChatRankKey 用户活跃度排名键名，用于存储用户聊天活跃度的有序集合
	ChatRankKey = "chat_activity_rank"
	//ChatGroupKey 聊天消息的消费者组键名
	ChatGroupKey = "chat_consumer_group"
)

type ChatMessage struct {
	Name    string // 发送者昵称
	Message string // 消息内容
	Type    string // 消息类型 ("chat" 或 "system")
}

// NewRedisQueueClient 创建一个新的 Redis 队列客户端实例。
// 参数:
//   - addr: Redis 服务器地址，格式如 "host:port"
//   - password: Redis 访问密码，若无则传空字符串
//   - queueKey: 指定用于存储队列数据的 Redis 键名
//   - db: 使用的 Redis 数据库编号
//
// 返回值:
//   - 成功时返回初始化后的 *RedisQueueClient 实例
//   - 若连接失败，则打印错误日志并返回 nil
func NewRedisQueueClient(addr, password string, db int) *RedisQueueClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("连接 Redis 失败（异步队列功能禁用）: %v", err)
		return nil // 连接失败，返回 nil
	}
	fmt.Println("Redis 连接成功")
	return &RedisQueueClient{
		Client:    rdb,
		StreamKey: TaskStreamKey,
		GroupKey:  ChatGroupKey,
	}
}

func (rqc *RedisQueueClient) AsyncProduceMessage(msg *ChatMessage) error {
	if rqc == nil || rqc.Client == nil {
		return fmt.Errorf("redis队列客户端未初始化")
	}
	ctx := context.Background()

	err := rqc.Client.XAdd(ctx, &redis.XAddArgs{
		Stream: ChatStreamKey,
		MaxLen: 1000,
		Values: map[string]interface{}{
			"sender":    msg.Name,
			"context":   msg.Message,
			"type":      msg.Type,
			"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("消息异步入队失败: %v", err)
	}
	return nil
}

func (rqc *RedisQueueClient) CreateChatConsumerGroup() error {
	if rqc == nil || rqc.Client == nil {
		return fmt.Errorf("redis 队列客户端未初始化")
	}
	ctx := context.Background()
	err := rqc.Client.XGroupCreateMkStream(ctx, ChatStreamKey, ChatGroupKey, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSY GROUP") {
		return fmt.Errorf("创建 Redis Stream 消费者组失败: %v", err)
	}
	return nil
}

func (rqc *RedisQueueClient) StartChatConsumer(consumerName string, handler func(msg *ChatMessage)) {
	ctx := context.Background()

	go func() {
		for {
			streams, err := rqc.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ChatGroupKey,
				Consumer: consumerName,
				Streams:  []string{ChatStreamKey, ">"},
				Count:    1,
				Block:    time.Second,
			}).Result()
			if err != nil {
				if err != redis.Nil {
					log.Printf("消费者%s读取stream失败：%v", consumerName, err)
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			for _, stream := range streams {
				for _, message := range stream.Messages {
					values := message.Values
					chatMsg := &ChatMessage{
						Name:    values["sender"].(string),
						Message: values["context"].(string),
						Type:    values["type"].(string),
					}
					if chatMsg.Type != "system" {
						handler(chatMsg)
					}
					if err = rqc.Client.XAck(ctx, ChatStreamKey, ChatGroupKey, message.ID).Err(); err != nil {
						log.Printf("消费者 %s ACK 消息 %s 失败: %v", consumerName, message.ID, err)
					}
				}
			}
		}
	}()
}

// GetChatHistory 获取聊天历史记录
// 参数:
//
//	count: 要获取的历史记录数量
//
// 返回值:
//
//	[]string: 聊天历史记录列表，按时间顺序排列
//	error: 错误信息，如果获取失败则返回错误
func (rqc *RedisQueueClient) GetChatHistory(count int64) ([]string, error) {
	if rqc == nil || rqc.Client == nil {
		return nil, fmt.Errorf("redis 队列客户端未初始化")
	}
	ctx := context.Background()

	// 从Redis中逆序读取指定数量的聊天记录
	streams, err := rqc.Client.XRevRangeN(ctx, ChatStreamKey, "+", "-", count).Result()
	if err != nil {
		return nil, fmt.Errorf("读取聊天历史失败:%v", err)
	}

	// 解析聊天记录并按正确的时间顺序组装消息
	var history []string
	for i := len(streams) - 1; i >= 0; i-- {
		stream := streams[i]
		sender := stream.Values["sender"]
		content := stream.Values["context"]
		timestamp := stream.Values["timestamp"]
		msg := fmt.Sprintf("[%s] %s: %s", timestamp, sender, content)
		history = append(history, msg)
	}
	return history, nil
}

// IncrUserAction 增加用户活跃度计数
// 该函数通过将指定用户名在Redis有序集合中的分数加1来记录用户活跃度
// 参数:
//   - username: 需要增加活跃度的用户名
//
// 返回值:
//   - error: 操作成功返回nil，否则返回具体错误信息
func (rqc *RedisQueueClient) IncrUserAction(username string) error {
	if rqc == nil || rqc.Client == nil {
		return fmt.Errorf("redis 队列客户端未初始化")
	}
	ctx := context.Background()

	// 使用ZIncrBy命令将用户名对应的分数加1，实现活跃度计数
	err := rqc.Client.ZIncrBy(ctx, ChatRankKey, 1, username).Err()
	if err != nil {
		return fmt.Errorf("增加用户活跃度失败：%v", err)
	}
	return nil
}

// GetActivityRank 获取用户活跃度排名
// 该函数从Redis有序集合中获取指定数量的用户活跃度排名信息
// 参数:
//   - count: 需要获取的排名数量
//
// 返回值:
//   - []string: 包含排名信息的字符串切片，格式为"Rank X:用户名(消息数：Y)"
//   - error: 操作成功返回nil，否则返回具体错误信息
func (rqc *RedisQueueClient) GetActivityRank(count int64) ([]string, error) {
	if rqc == nil || rqc.Client == nil {
		return nil, fmt.Errorf("redis队列客户端未初始化")
	}
	ctx := context.Background()

	// 使用ZRevRangeWithScores命令获取按分数降序排列的用户排名数据
	results, err := rqc.Client.ZRevRangeWithScores(ctx, ChatRankKey, 0, count-1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取活跃度排名失败：%w", err)
	}

	// 格式化排名结果为可读字符串
	var rankList []string
	for i, z := range results {
		rank := i + 1
		username := z.Member.(string)
		score := int(z.Score)
		rankEntry := fmt.Sprintf("Rank %d:%s(消息数：%d)", rank, username, score)
		rankList = append(rankList, rankEntry)
	}
	return rankList, nil
}
