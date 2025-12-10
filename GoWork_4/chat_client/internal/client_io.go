package internal

import (
	"GoWork_4/tools"
	"fmt"
)

// safeReceiveFromServer 在独立协程中安全地从服务器接收数据。
// 使用select监听done通道以优雅关闭该协程。
// 若接收到的消息出错会通过errorChan传递错误。
func (c *Client) safeReceiveFromServer() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("接收协程发生panic: %v\n", r)
		}
	}()

	for c.isConnectedAtomic() {
		select {
		case <-c.done:
			return
		default:
			msg, err := tools.ReceiveMessage(c.conn)
			if err != nil {
				select {
				case c.errorChan <- fmt.Errorf("与服务器断开连接: %v", err):
				default:
				}
				return
			}

			select {
			case c.receiveChan <- msg:
			case <-c.done:
				return
			}
		}
	}
}

// safeSendToServer 在独立协程中安全地向服务器发送数据。
// 监听sendChan中的消息并通过网络连接发送出去。
// 出现发送错误时将错误放入errorChan。
func (c *Client) safeSendToServer() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("发送协程发生panic: %v\n", r)
		}
	}()

	for {
		select {
		case <-c.done:
			return
		case msg, ok := <-c.sendChan:
			if !ok {
				return
			}
			if err := tools.SendMessage(c.conn, msg); err != nil {
				select {
				case c.errorChan <- fmt.Errorf("发送消息失败: %v", err):
				default:
				}
				return
			}
		}
	}
}

// safeHandleMessages 在独立协程中处理来自服务器的消息以及错误事件。
// 当收到消息时调用工具函数打印出来并在控制台输出提示符。
// 收到错误后调用handleConnectionError方法处理异常情况。
func (c *Client) safeHandleMessages() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("消息处理协程发生panic: %v\n", r)
		}
	}()

	for {
		select {
		case <-c.done:
			return
		case msg, ok := <-c.receiveChan:
			if !ok {
				return
			}
			// 使用tools包的PrintMessage显示消息
			tools.PrintMessage("", msg)

		case err, ok := <-c.errorChan:
			if !ok {
				return
			}
			c.handleConnectionError(err) // handleConnectionError 在 client.go 中
			return
		}
	}
}

// userInputLoop 主线程中运行的用户交互循环。
// 不断等待用户输入并将有效内容放入sendChan中。
// 输入“/exit”命令可终止程序。
func (c *Client) userInputLoop() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("接收协程发生panic: %v\n", r)
		}
	}()
	for c.isConnectedAtomic() {
		select {
		case <-c.done:
			return
		default:
			input, err := tools.ReadInput("")
			if err != nil {
				continue
			}

			if input == "/exit" {
				fmt.Println("再见！")
				c.cleanup()
				return
			}

			if input == "" {
				continue
			}

			select {
			case c.sendChan <- input:
			case <-c.done:
				return
			default:
				fmt.Println("发送队列已满，请稍后再试")
			}
		}
	}
}
