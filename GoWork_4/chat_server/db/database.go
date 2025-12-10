package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

type UserDB struct {
	DB *sql.DB
}

func ConnectDB() *UserDB {
	connStr := "root:231792@tcp(localhost:3306)/User"
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		err = fmt.Errorf("数据库打开失败：%v\n", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		fmt.Printf("数据库连接失败：%v\n", err)
		db.Close()
		return nil
	}
	fmt.Println("数据库连接成功")
	return &UserDB{DB: db}

}

// CheckNameExists 检查用户名是否已在数据库中存在。
func (udb *UserDB) CheckNameExists(name string) (bool, error) {
	if udb.DB == nil {
		return false, fmt.Errorf("数据库连接不可用")
	}

	var count int
	// 假设您的表名为 'users'，字段名为 'username'
	err := udb.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", name).Scan(&count)

	if (err != nil) && (err != sql.ErrNoRows) {
		return false, fmt.Errorf("查询用户名失败: %v", err)
	}

	return count > 0, nil
}

// RegisterUser 将新用户（昵称和密码）写入数据库（注册功能）。
// 它会对密码进行哈希处理后存储
func (udb *UserDB) RegisterUser(name, password string) error {
	if udb.DB == nil {
		return fmt.Errorf("数据库来连接不可用")
	}
	stmt, err := udb.DB.Prepare("INSERT INTO users (username,password_hash,created_at) value (?,?,NOW())")
	if err != nil {
		return fmt.Errorf("准备SQL语句失败：%v", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(name, password)
	if err != nil {
		return fmt.Errorf("执行插入操作失败：%v", err)
	}
	fmt.Printf("用户注册成功：\n")
	return nil
}

// CheckCredentials 检查用户名和密码是否匹配
// 流程：从数据库
// 返回值：（是否验证成功，错误信息）
func (udb *UserDB) CheckCredentials(name, password string) (bool, error) {
	if udb.DB == nil {
		return false, fmt.Errorf("数据库连接不可用")
	}
	var storedPassword string
	err := udb.DB.QueryRow("select password_hash from users where	username = ?", name).Scan(&storedPassword)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询用户凭证失败：%v", err)
	}

	if password == storedPassword {
		return true, nil
	} else {
		return false, nil
	}

}

// Close 关闭数据库连接
func (udb *UserDB) Close() {
	if udb.DB != nil {
		udb.DB.Close()
		fmt.Println("[DB] 数据库连接已关闭。")
	}
}
