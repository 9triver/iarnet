# 架构优化建议：Manager 命名与依赖关系

## 当前架构分析

### 1. 命名层次
- **顶层 Manager** (`resource.Manager`, `application.Manager`)
  - 职责：聚合子模块的 Service，提供统一的对外接口
  - 特点：不直接管理状态，而是委托给子 Service
  - 问题：命名为 "Manager" 但实际更像 "Facade" 或 "Service"

- **子模块 Manager** (`runner.Manager`, `workspace.Manager`, `provider.Manager`)
  - 职责：管理内存中的领域对象状态
  - 特点：有状态，负责对象的生命周期管理
  - 命名：符合 "Manager" 的含义

- **子模块 Service** (`runner.Service`, `workspace.Service`, `provider.Service`)
  - 职责：提供无状态的业务逻辑
  - 特点：依赖 Manager 获取领域对象，执行业务操作
  - 命名：符合 "Service" 的含义

### 2. 依赖关系
```
Transport Layer
    ↓
顶层 Manager (resource.Manager, application.Manager)
    ↓
子模块 Service (runner.Service, workspace.Service, ...)
    ↓
子模块 Manager (runner.Manager, workspace.Manager, ...)
    ↓
领域对象 (Runner, Workspace, ...)
```

## 优化方案

### 方案一：重命名为 Service（推荐）

**优点**：
- 更准确地反映职责：顶层对象对外提供服务
- 与子模块的 Service 命名一致，形成清晰的层次
- 符合 DDD 中 "Application Service" 的概念

**缺点**：
- 需要大量重命名，影响范围广
- 可能与子模块 Service 产生命名冲突（可通过包名区分）

**实现**：
```go
// 顶层：Application Service
package application
type Service struct {
    runnerSvc    runner.Service
    workspaceSvc workspace.Service
    metadataSvc  metadata.Service
}

// 使用
applicationService := application.NewService(...)
```

### 方案二：保持 Manager，但明确职责

**优点**：
- 改动最小，只需添加注释和文档
- 保持现有命名习惯

**缺点**：
- 命名仍然可能造成混淆
- 需要额外的文档说明

**实现**：
```go
// Manager 是模块级别的服务聚合器（Facade）
// 它不直接管理状态，而是聚合子模块的 Service
// 子模块的 Manager 负责状态管理
type Manager struct {
    runnerSvc    runner.Service
    workspaceSvc workspace.Service
    metadataSvc  metadata.Service
}
```

### 方案三：引入 Facade 模式（不推荐）

**优点**：
- 明确表示这是门面模式
- 职责清晰

**缺点**：
- 引入新的命名概念
- 与现有架构风格不一致

## 推荐方案：方案一（重命名为 Service）

### 理由：
1. **语义准确**：顶层对象确实是在提供服务，而不是管理状态
2. **层次清晰**：
   - 顶层：`application.Service`（应用服务）
   - 子模块：`runner.Service`（领域服务）
   - 状态管理：`runner.Manager`（领域对象管理器）
3. **符合 DDD**：顶层 Service 对应 Application Service，子模块 Service 对应 Domain Service

### 重构步骤：
1. 重命名顶层 Manager 为 Service
2. 更新所有引用
3. 更新文档和注释

### 命名规范：
- **顶层 Service**：`application.Service`, `resource.Service`
- **子模块 Service**：`runner.Service`, `workspace.Service`
- **子模块 Manager**：`runner.Manager`, `workspace.Manager`
- **领域对象**：`Runner`, `Workspace`

## 依赖关系优化

### 当前依赖关系（合理）
```
Transport → 顶层 Manager/Service → 子模块 Service → 子模块 Manager → 领域对象
```

### 建议：
1. **保持当前依赖方向**：依赖关系是合理的，符合依赖倒置原则
2. **明确接口定义**：顶层 Service 应该实现明确的接口，而不是通过接口断言
3. **避免循环依赖**：确保依赖方向是单向的

## 总结

**推荐采用方案一**：将顶层 `Manager` 重命名为 `Service`，这样：
- 命名更准确
- 层次更清晰
- 符合 DDD 实践
- 与子模块命名风格一致

**如果不想大规模重构**：采用方案二，保持现有命名，但添加清晰的注释和文档说明顶层 Manager 的实际职责。

