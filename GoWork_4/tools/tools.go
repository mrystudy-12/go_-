package tools

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// 消息头长度（4字节，存储消息体长度）
const headerSize = 4

// SendMessage 发送带长度的消息（解决粘包）
func SendMessage(conn net.Conn, message string) error {
	// 将消息转换为字节
	body := []byte(message)
	bodyLen := len(body)

	// 创建数据包：[4字节长度][消息体]
	packet := make([]byte, headerSize+bodyLen)

	// 写入消息长度（大端序）
	binary.BigEndian.PutUint32(packet[:headerSize], uint32(bodyLen))

	// 写入消息体
	copy(packet[headerSize:], body)

	// 发送完整数据包
	_, err := conn.Write(packet)
	if err != nil {
		// 添加详细错误日志记录
		netErr, ok := err.(net.Error)
		if ok {
			if netErr.Timeout() {
				fmt.Printf("发送消息超时: %s, 消息长度: %d\n", message[:min(len(message), 50)], bodyLen)
			} else if netErr.Temporary() {
				fmt.Printf("临时网络错误: %v, 消息长度: %d\n", netErr, bodyLen)
			}
		}
		// 记录基础错误信息
		fmt.Printf("发送消息失败: %v, 消息: %.50s..., 长度: %d\n", err, message, bodyLen)
		return err
	}
	return nil
}

// ReceiveMessage 接收带长度的消息（解决粘包）
func ReceiveMessage(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)

	// 1. 先读取消息头（4字节长度）
	header := make([]byte, headerSize)
	_, err := io.ReadFull(reader, header)
	if err != nil {
		return "", err
	}

	// 2. 解析消息体长度
	bodyLen := binary.BigEndian.Uint32(header)

	// 3. 读取消息体
	body := make([]byte, bodyLen)
	_, err = io.ReadFull(reader, body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// PrintMessage 打印消息
func PrintMessage(prefix, msg string) {
	fmt.Printf("%s%s\n", prefix, msg)
}

// ReadInput 读取用户输入（支持空格）
func ReadInput(prompt string) (string, error) {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", scanner.Err()
}
