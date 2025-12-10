package internal

import (
	"GoWork_4/tools"
	"fmt"
	"strings"
)

// handleAuthentication 处理用户的登录/注册选择和流程。
// 客户端需要在一个主循环中等待服务器的提示，并根据提示进行输入。
func (c *Client) handleAuthentication() error {
	// 1. 接收服务器的欢迎消息 (第一次进来时，服务器发送了 "欢迎！请选择操作：...")
	msg, err := tools.ReceiveMessage(c.conn)
	if err != nil {
		return fmt.Errorf("接收欢迎消息失败: %v", err)
	}

	for { // 主认证循环
		fmt.Println(msg)
		// 1. 处理选项选择 (1/2)
		nextPrompt, currentName, err := c.handleSelection()
		if err != nil {
			return err // 选项输入/发送失败
		}
		// 【修复点 2：检查并跳过重复的主菜单提示】
		// 如果服务器在选项选择后错误地再次发送了主菜单，我们将使用下一步的提示覆盖它。
		if strings.Contains(nextPrompt, "欢迎！请选择操作：") {
			// 如果 nextPrompt 仍然是主菜单，说明服务器发送重复了。
			// 客户端需要再次接收下一步的提示（例如 "请输入昵称："）
			newNextPrompt, err := tools.ReceiveMessage(c.conn)
			if err != nil {
				return fmt.Errorf("接收下一步提示失败: %v", err)
			}
			nextPrompt = newNextPrompt // 使用正确的下一步提示
		}
		// 如果服务器拒绝了选项 (如“无效的选项”)，nextPrompt 就是错误消息，继续外层主循环
		if strings.Contains(nextPrompt, "无效的选项") {
			fmt.Println(nextPrompt) // 更新提示，重新显示主菜单和错误
			continue
		}

		// 2. 处理昵称输入和验证 (昵称循环)
		// nextPrompt 此时是 "请输入昵称" 或重试提示
		resultPrompt, name, err := c.handleNicknameInput(nextPrompt)
		if err != nil {
			return err // 昵称输入/发送失败
		}
		currentName = name // 更新客户端的昵称

		// 检查昵称流程是否要求返回主菜单
		if strings.Contains(resultPrompt, "请返回主菜单") || strings.Contains(resultPrompt, "未注册") {
			// 打印服务器提示，然后重新接收主菜单提示，回到主循环顶部
			fmt.Println(resultPrompt)
			newMsg, err := tools.ReceiveMessage(c.conn) // 重新接收 "欢迎！请选择操作..."
			if err != nil {
				return fmt.Errorf("接收主菜单提示失败: %v", err)
			}
			msg = newMsg
			continue
		}

		// 3. 处理密码输入和验证 (密码循环)
		// resultPrompt 此时是 "请输入密码" 提示
		finalMsg, err := c.handlePasswordInput(resultPrompt, currentName)
		if err != nil {
			return err // 密码输入/发送失败
		}

		// 检查最终结果
		if strings.Contains(finalMsg, "开始聊天") {
			// 登录成功
			fmt.Println(finalMsg)
			return nil
		}

		// 注册成功或登录失败次数过多，要求返回主菜单
		if strings.Contains(finalMsg, "请返回主菜单") {
			fmt.Println(finalMsg)
			newMsg, err := tools.ReceiveMessage(c.conn) // 重新接收 "欢迎！请选择操作..."
			if err != nil {
				return fmt.Errorf("接收主菜单提示失败: %v", err)
			}
			msg = newMsg
			continue // 回到主循环顶部，重新选择操作
		}

		// 收到意外响应，流程错误
		return fmt.Errorf("认证流程中收到意外的服务器响应: %s", finalMsg)
	} // 结束主循环
}

func (c *Client) handleSelection() (string, string, error) {
	selection, err := tools.ReadInput("请输入选项（1/2）：")
	if err != nil {
		return "", "", fmt.Errorf("读取选项失败：%v", err)
	}
	if err := tools.SendMessage(c.conn, selection); err != nil {
		return "", "", fmt.Errorf("发送选项失败：%v", err)
	}
	nextPrompt, err := tools.ReceiveMessage(c.conn)
	if err != nil {
		return "", "", fmt.Errorf("接收子流程提示失败：%v", err)
	}
	return nextPrompt, "", nil
}

func (c *Client) handleNicknameInput(nextPrompt string) (string, string, error) {
	currentName := ""
	for {
		if strings.Contains(nextPrompt, "请返回主菜单") || strings.Contains(nextPrompt, "未注册") {
			return nextPrompt, currentName, nil
		}
		fmt.Println(nextPrompt)
		name, err := tools.ReadInput("")
		if err != nil {
			return "", "", fmt.Errorf("读取昵称输入失败;%v", err)
		}
		currentName = name
		if err := tools.SendMessage(c.conn, name); err != nil {
			return "", "", fmt.Errorf("发动昵称失败：%v", err)
		}
		response, err := tools.ReceiveMessage(c.conn)
		if err != nil {
			return "", "", fmt.Errorf("接收昵称失败：%v", err)
		}
		if strings.Contains(response, "请重新输入昵称") || strings.Contains(response, "昵称无效") || strings.Contains(response, "已在线") {
			nextPrompt = response
			continue
		}
		// 昵称通过校验，内容是密码提示，或要求返回主菜单。跳出循环。
		return response, currentName, nil

	}
}

func (c *Client) handlePasswordInput(nextPrompt string, currentName string) (string, error) {
	// 打印“请输入密码进行登录/注册”
	fmt.Println(nextPrompt)

	for {
		password, err := tools.ReadInput("")
		if err != nil {
			return "", fmt.Errorf("读取密码输入失败: %v", err)
		}

		if err := tools.SendMessage(c.conn, password); err != nil {
			return "", fmt.Errorf("发送密码失败: %v", err)
		}

		// 接收最终结果 (成功/失败重试/返回主菜单)
		finalResponse, err := tools.ReceiveMessage(c.conn)
		if err != nil {
			return "", fmt.Errorf("接收最终响应失败: %v", err)
		}

		// 检查是否成功进入聊天室 (仅登录成功)
		if strings.Contains(finalResponse, "开始聊天") {
			c.name = currentName      // 设置客户端昵称
			return finalResponse, nil // 成功，返回给主函数处理退出
		}

		// 检查是否是注册成功或登录失败次数过多，要求返回选择界面
		if strings.Contains(finalResponse, "请返回主菜单") {
			return finalResponse, nil // 返回主菜单，返回给主函数处理退出
		}

		// 处理认证失败或重试提示
		fmt.Println(finalResponse) // 打印失败原因 (如 "密码不正确")

		retryPrompt, err := tools.ReceiveMessage(c.conn)
		if err != nil {
			return "", fmt.Errorf("接收重试提示失败: %v", err)
		}

		if strings.Contains(retryPrompt, "请重新输入密码") {
			fmt.Println(retryPrompt)
			continue // 继续密码循环
		} else {
			return "", fmt.Errorf("认证流程中收到意外的服务器响应: %s", finalResponse)
		}
	}
}
