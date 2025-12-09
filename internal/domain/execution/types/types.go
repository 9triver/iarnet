package types

import (
	resourcetypes "github.com/9triver/iarnet/internal/domain/resource/types"
)

// Execution 领域类型
// execution 依赖 resource，可以使用 resource/types 的类型

// ActorID Actor 标识符
type ActorID = string

// SessionID 会话标识符
type SessionID = string

// RuntimeID 运行时标识符
type RuntimeID = string

// 重新导出 resource 领域类型以便使用
type Info = resourcetypes.Info
type Capacity = resourcetypes.Capacity
type ResourceRequest = resourcetypes.ResourceRequest
type RuntimeEnv = resourcetypes.RuntimeEnv
type ObjectID = resourcetypes.ObjectID
type StoreID = resourcetypes.StoreID

// 重新导出 resource 常量
const (
	RuntimeEnvPython = resourcetypes.RuntimeEnvPython
)
