#!/bin/bash

# 读取 users 表数据的脚本
# 用法: ./read_users.sh [数据库文件路径]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DB_PATH="${1:-$PROJECT_ROOT/data/users.db}"

cd "$PROJECT_ROOT"

# 检查数据库文件是否存在
if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database file not found: $DB_PATH" >&2
    exit 1
fi

# 运行 Go 脚本
go run "$SCRIPT_DIR/read_users.go" "$DB_PATH"
