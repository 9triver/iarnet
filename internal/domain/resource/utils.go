package resource

import "time"

// getCurrentTimestamp 获取当前时间戳（全局函数）
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}
