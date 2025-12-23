package types

import "time"

// OperationType 操作类型
type OperationType string

const (
	OperationTypeCreateApplication OperationType = "create_application"
	OperationTypeUpdateApplication OperationType = "update_application"
	OperationTypeDeleteApplication OperationType = "delete_application"
	OperationTypeRunApplication    OperationType = "run_application"
	OperationTypeStopApplication   OperationType = "stop_application"
	OperationTypeCreateFile        OperationType = "create_file"
	OperationTypeUpdateFile        OperationType = "update_file"
	OperationTypeDeleteFile        OperationType = "delete_file"
	OperationTypeCreateDirectory   OperationType = "create_directory"
	OperationTypeDeleteDirectory   OperationType = "delete_directory"
	OperationTypeRegisterResource  OperationType = "register_resource"
	OperationTypeUpdateResource    OperationType = "update_resource"
	OperationTypeDeleteResource    OperationType = "delete_resource"
)

// OperationLog 操作日志
type OperationLog struct {
	ID           string                 `json:"id"`
	User         string                 `json:"user"`          // 操作用户
	Operation    OperationType          `json:"operation"`     // 操作类型
	ResourceID   string                 `json:"resource_id"`   // 资源ID（如应用ID）
	ResourceType string                 `json:"resource_type"` // 资源类型（如 "application"）
	Action       string                 `json:"action"`        // 操作描述
	Before       map[string]interface{} `json:"before"`        // 操作前的状态
	After        map[string]interface{} `json:"after"`         // 操作后的状态
	Timestamp    time.Time              `json:"timestamp"`     // 操作时间
	IP           string                 `json:"ip,omitempty"`  // 操作IP地址
}

// QueryOptions 查询选项
type QueryOptions struct {
	StartTime  *time.Time    `json:"start_time,omitempty"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	User       string        `json:"user,omitempty"`
	Operation  OperationType `json:"operation,omitempty"`
	ResourceID string        `json:"resource_id,omitempty"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

// QueryResult 查询结果
type QueryResult struct {
	Logs    []*OperationLog `json:"logs"`
	Total   int             `json:"total"`
	HasMore bool            `json:"has_more"`
}
