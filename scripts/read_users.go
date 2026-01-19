package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID        string
	Name      string
	Password  string
	Role      string
	CreatedAt string
	UpdatedAt string
}

func main() {
	// 数据库文件路径
	dbPath := "data/users.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// 打开数据库
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=1&_journal_mode=WAL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}

	// 查询数据
	query := `
		SELECT id, name, password, role, 
		       datetime(created_at) as created_at, 
		       datetime(updated_at) as updated_at
		FROM users
		ORDER BY created_at ASC
	`

	rows, err := db.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying database: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Password, &user.Role, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			continue
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error iterating rows: %v\n", err)
		os.Exit(1)
	}

	// 输出表格格式
	printTable(users)
}

func printTable(users []User) {
	if len(users) == 0 {
		fmt.Println("Empty set (0.00 sec)")
		return
	}

	// 定义列
	columns := []struct {
		name string
		width int
	}{
		{"id", 0},
		{"name", 0},
		{"password", 0},
		{"role", 0},
		{"created_at", 0},
		{"updated_at", 0},
	}

	// 计算每列的最大宽度
	for i := range columns {
		columns[i].width = len(columns[i].name)
		for _, user := range users {
			var value string
			switch i {
			case 0:
				value = user.ID
			case 1:
				value = user.Name
			case 2:
				// 密码只显示前20个字符，后面用...表示
				if len(user.Password) > 20 {
					value = user.Password[:20] + "..."
				} else {
					value = user.Password
				}
			case 3:
				value = user.Role
			case 4:
				value = user.CreatedAt
			case 5:
				value = user.UpdatedAt
			}
			if len(value) > columns[i].width {
				columns[i].width = len(value)
			}
		}
		// 最小宽度
		if columns[i].width < 4 {
			columns[i].width = 4
		}
	}

	// 打印表头
	printSeparator(columns)
	printHeader(columns)
	printSeparator(columns)

	// 打印数据行
	for _, user := range users {
		values := []string{
			truncateString(user.ID, columns[0].width),
			truncateString(user.Name, columns[1].width),
			truncatePassword(user.Password, columns[2].width),
			truncateString(user.Role, columns[3].width),
			truncateString(user.CreatedAt, columns[4].width),
			truncateString(user.UpdatedAt, columns[5].width),
		}
		fmt.Printf("| %s |\n", strings.Join(values, " | "))
	}

	// 打印底部分隔线
	printSeparator(columns)

	// 打印统计信息
	fmt.Printf("%d row%s in set (0.00 sec)\n", len(users), plural(len(users)))
}

func printSeparator(columns []struct{name string; width int}) {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = strings.Repeat("-", col.width+2)
	}
	fmt.Printf("+%s+\n", strings.Join(parts, "+"))
}

func printHeader(columns []struct{name string; width int}) {
	parts := make([]string, len(columns))
	for i, col := range columns {
		parts[i] = fmt.Sprintf(" %-*s ", col.width, col.name)
	}
	fmt.Printf("|%s|\n", strings.Join(parts, "|"))
}

func truncateString(s string, width int) string {
	if len(s) > width {
		return s[:width-3] + "..."
	}
	return fmt.Sprintf("%-*s", width, s)
}

func truncatePassword(password string, width int) string {
	// 密码显示策略：如果超过宽度，显示前部分+...
	if len(password) > width {
		if width > 20 {
			return password[:width-3] + "..."
		}
		return strings.Repeat("*", width)
	}
	return fmt.Sprintf("%-*s", width, password)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
